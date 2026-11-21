//go:build darwin && !ios

package kiosk

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/HeroSizy/baby-piano/piano"
)

// Supported reports whether this platform locks the keyboard.
func Supported() bool { return true }

// SecureInput reports whether another app turned on secure keyboard entry,
// as a password field or a terminal can, which no app can lock through.
func SecureInput() bool {
	load()
	return isSecureEventInputEnabled()
}

// ErrNotAllowed is Start's error when the system refused the keyboard.
var ErrNotAllowed = errors.New("kiosk: the system refused the keyboard; check Accessibility in System Settings")

// lock is the running event tap. The tap's callback is a C function with no
// context, so the lock is the package's one.
var lock struct {
	sync.Mutex
	tap, source uintptr
	onKey       func(Key)
	hideCursor  bool
}

// Start swallows every key, media key and trackpad gesture, and with
// opts.LockMouse the mouse too, passing each key pressed (not repeated) to
// onKey on a goroutine of its own. It runs until Stop.
func Start(opts Options, onKey func(Key)) error {
	load()
	lock.Lock()
	defer lock.Unlock()
	if lock.tap != 0 {
		return nil
	}
	// 10 key down, 11 key up, 12 modifiers, 14 system-defined (media,
	// brightness and volume keys), 18–20 and 29–32 trackpad gestures, so a
	// swipe neither changes Space nor opens Mission Control.
	var mask uint64
	for _, t := range []uint{10, 11, 12, 14, 18, 19, 20, 29, 30, 31, 32} {
		mask |= 1 << t
	}
	if opts.LockMouse {
		// Buttons, moves, drags, scrolls: the pointer stays put and hot
		// corners never fire.
		for _, t := range []uint{1, 2, 3, 4, 5, 6, 7, 22, 25, 26, 27} {
			mask |= 1 << t
		}
	}
	const hidEventTap, headInsert, defaultTap = 0, 0, 0
	tap := cgEventTapCreate(hidEventTap, headInsert, defaultTap, mask, tapCallback, 0)
	if tap == 0 {
		return ErrNotAllowed
	}
	// The tap's callback runs on the main thread, in the run loop AppKit
	// serves: reading a key's text or making an NSEvent asks the input
	// source, which macOS wants on the main thread only, as Caps Lock
	// switching the input source showed.
	source := cfMachPortCreateRunLoopSource(0, tap, 0)
	cfRunLoopAddSource(cfRunLoopGetMain(), source, symValue(coreFoundation, "kCFRunLoopCommonModes"))
	cgEventTapEnable(tap, true)
	lock.tap, lock.source, lock.onKey, lock.hideCursor = tap, source, onKey, opts.LockMouse
	return nil
}

// Stop gives the keyboard back.
func Stop() {
	lock.Lock()
	defer lock.Unlock()
	if lock.tap == 0 {
		return
	}
	cgEventTapEnable(lock.tap, false)
	cfRunLoopRemoveSource(cfRunLoopGetMain(), lock.source, symValue(coreFoundation, "kCFRunLoopCommonModes"))
	cfMachPortInvalidate(lock.tap)
	cfRelease(lock.source)
	cfRelease(lock.tap)
	if lock.hideCursor {
		onMain(func() { cgDisplayShowCursor(cgMainDisplayID()) })
	}
	lock.tap, lock.source, lock.onKey, lock.hideCursor = 0, 0, nil, false
}

// Tick keeps the lock and the volume cap in force; call it every half
// second while playing. It turns the tap back on if the system turned it off
// and lowers the volume past MaxSystemVolume, and reports whether the output
// device changed since the last tick, as a Bluetooth speaker connecting, so
// the app can hold its notes a moment.
func Tick() (outputChanged bool) {
	load()
	lock.Lock()
	if lock.tap != 0 && !cgEventTapIsEnabled(lock.tap) {
		cgEventTapEnable(lock.tap, true)
	}
	lock.Unlock()
	dev := defaultOutputDevice()
	changed := dev != lastDevice && lastDevice != 0
	lastDevice = dev
	if v, ok := SystemVolume(); ok && v > MaxSystemVolume {
		setSystemVolume(MaxSystemVolume)
	}
	return changed
}

var lastDevice uint32

// tapCallback is the event tap's C callback: it hands keys to the app and
// swallows every event it hears.
var tapCallback = purego.NewCallback(func(proxy, typ, event, refcon uintptr) uintptr {
	const disabledByTimeout, disabledByUserInput = 0xFFFFFFFE, 0xFFFFFFFF
	switch uint32(typ) {
	case disabledByTimeout, disabledByUserInput:
		// The system turns a slow tap off; turn it straight back on.
		lock.Lock()
		if lock.tap != 0 {
			cgEventTapEnable(lock.tap, true)
		}
		lock.Unlock()
		return event
	case 10: // key down
		const autorepeat, keycode = 8, 9
		if cgEventGetIntegerValueField(event, autorepeat) != 0 {
			return 0 // a held key plays once
		}
		var buf [4]uint16
		var n uint64
		cgEventKeyboardGetUnicodeString(event, 4, &n, &buf[0])
		fire(Key{Name: piano.MacKeyName(int(cgEventGetIntegerValueField(event, keycode))), Text: utf16String(buf[:n])})
	case 12: // a modifier down or up; Caps Lock toggles
		const keycode = 9
		code := int(cgEventGetIntegerValueField(event, keycode))
		if modifierDown(code, cgEventGetFlags(event)) {
			fire(Key{Name: piano.MacKeyName(code)})
		}
	case 14: // system-defined: media, brightness, volume, eject
		pool := objcAutoreleasePoolPush()
		e := objc.ID(objc.GetClass("NSEvent")).Send(objc.RegisterName("eventWithCGEvent:"), event)
		if e != 0 && objc.Send[int16](e, objc.RegisterName("subtype")) == 8 {
			data := objc.Send[int](e, objc.RegisterName("data1"))
			keyType, flags := (data>>16)&0xFFFF, data&0xFFFF
			if (flags>>8)&0xFF == 0x0A && flags&1 == 0 { // down, not a repeat
				fire(Key{Name: piano.MacMediaKeyName(keyType)})
			}
		}
		objcAutoreleasePoolPop(pool)
	}
	return 0
})

func fire(k Key) {
	lock.Lock()
	onKey := lock.onKey
	lock.Unlock()
	if onKey != nil {
		go onKey(k)
	}
}

// modifierDown tells a modifier going down from one coming up by the flags
// the event carries.
func modifierDown(code int, flags uint64) bool {
	const shift, control, alternate, command, fn = 1 << 17, 1 << 18, 1 << 19, 1 << 20, 1 << 23
	switch code {
	case 56, 60:
		return flags&shift != 0
	case 59, 62:
		return flags&control != 0
	case 58, 61:
		return flags&alternate != 0
	case 55, 54:
		return flags&command != 0
	case 63:
		return flags&fn != 0
	case 57: // Caps Lock: each press
		return true
	}
	return false
}

func utf16String(u []uint16) string {
	r := make([]rune, 0, len(u))
	for _, c := range u {
		r = append(r, rune(c)) // the exit word's letters are ASCII
	}
	return string(r)
}

// Enter covers every screen: the piano's window becomes a borderless window
// over the screen saver's level on the main screen, the other screens get
// black windows, and the Dock, the menu bar, app switching and the
// shut-down dialog go away until the app quits. Force Quit stays, so a
// parent can still end a hung piano with Cmd+Option+Esc, which the lock
// swallows while the piano runs.
func Enter() {
	load()
	onMain(func() {
		app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
		screens := objc.ID(objc.GetClass("NSScreen")).Send(objc.RegisterName("screens"))
		main := objc.ID(objc.GetClass("NSScreen")).Send(objc.RegisterName("mainScreen"))
		win := mainWindow(app)
		const borderless, screenSaverLevel = 0, 1000
		const joinAllSpaces, stationary, ignoresCycle = 1 << 0, 1 << 4, 1 << 6
		cover := func(w objc.ID, frame nsRect) {
			w.Send(objc.RegisterName("setStyleMask:"), uint(borderless))
			w.Send(objc.RegisterName("setLevel:"), int(screenSaverLevel))
			w.Send(objc.RegisterName("setCollectionBehavior:"), uint(joinAllSpaces|stationary|ignoresCycle))
			w.Send(objc.RegisterName("setFrame:display:"), frame, true)
		}
		if win != 0 && main != 0 {
			cover(win, objc.Send[nsRect](main, objc.RegisterName("frame")))
			win.Send(objc.RegisterName("makeKeyAndOrderFront:"), 0)
		}
		n := objc.Send[uint](screens, objc.RegisterName("count"))
		for i := range n {
			s := screens.Send(objc.RegisterName("objectAtIndex:"), i)
			if s == main {
				continue
			}
			frame := objc.Send[nsRect](s, objc.RegisterName("frame"))
			const buffered = 2
			black := objc.ID(objc.GetClass("NSWindow")).Send(objc.RegisterName("alloc")).Send(
				objc.RegisterName("initWithContentRect:styleMask:backing:defer:"), frame, uint(borderless), uint(buffered), false)
			black.Send(objc.RegisterName("setReleasedWhenClosed:"), false)
			black.Send(objc.RegisterName("setBackgroundColor:"), objc.ID(objc.GetClass("NSColor")).Send(objc.RegisterName("blackColor")))
			cover(black, frame)
			black.Send(objc.RegisterName("orderFront:"), 0)
		}
		app.Send(objc.RegisterName("activateIgnoringOtherApps:"), true)
		// Hide Dock and menu bar, no app switching, no Apple menu, no
		// shut-down dialog, no Hide.
		const hideDock, hideMenuBar, noAppleMenu, noSwitching, noSessionEnd, noHide = 1 << 1, 1 << 3, 1 << 4, 1 << 5, 1 << 7, 1 << 8
		app.Send(objc.RegisterName("setPresentationOptions:"), uint(hideDock|hideMenuBar|noAppleMenu|noSwitching|noSessionEnd|noHide))
		lock.Lock()
		hide := lock.hideCursor
		lock.Unlock()
		if hide && main != 0 {
			// The locked mouse moves nothing: park the pointer in the
			// piano's screen and hide it there until Stop, the app now
			// active as hiding wants. Quitting shows it again.
			f := objc.Send[nsRect](main, objc.RegisterName("frame"))
			cgWarpMouseCursorPosition(cgPoint{f.X + f.W/2, f.H / 2})
			cgDisplayHideCursor(cgMainDisplayID())
		}
	})
}

type cgPoint struct{ X, Y float64 }

// Front brings the piano back in front if the system or another app took
// it, as a notification or the keyboard setup assistant can.
func Front() {
	load()
	onMain(func() {
		app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
		if objc.Send[bool](app, objc.RegisterName("isActive")) {
			return
		}
		app.Send(objc.RegisterName("activateIgnoringOtherApps:"), true)
		if win := mainWindow(app); win != 0 {
			win.Send(objc.RegisterName("makeKeyAndOrderFront:"), 0)
		}
	})
}

type nsRect struct{ X, Y, W, H float64 }

// mainWindow is the app's window NSGo opened: its main window, or the first
// one it shows.
func mainWindow(app objc.ID) objc.ID {
	if w := app.Send(objc.RegisterName("mainWindow")); w != 0 {
		return w
	}
	windows := app.Send(objc.RegisterName("windows"))
	for i := range objc.Send[uint](windows, objc.RegisterName("count")) {
		w := windows.Send(objc.RegisterName("objectAtIndex:"), i)
		if objc.Send[bool](w, objc.RegisterName("isVisible")) {
			return w
		}
	}
	return 0
}

// onMain runs fn on the main thread, where AppKit wants its calls, through
// the main dispatch queue NSGo's run loop serves.
func onMain(fn func()) {
	mainCalls.Lock()
	mainCalls.next++
	id := mainCalls.next
	mainCalls.fns[id] = fn
	mainCalls.Unlock()
	dispatchAsyncF(mainQueue, id, mainTrampoline)
}

var mainCalls = struct {
	sync.Mutex
	next uintptr
	fns  map[uintptr]func()
}{fns: map[uintptr]func(){}}

var mainTrampoline = purego.NewCallback(func(id uintptr) {
	mainCalls.Lock()
	fn := mainCalls.fns[id]
	delete(mainCalls.fns, id)
	mainCalls.Unlock()
	if fn != nil {
		fn()
	}
})

// SystemVolume reads the default output device's volume, from 0 to 1, where
// the device has one the system controls (HDMI and some USB devices do not).
func SystemVolume() (float32, bool) {
	load()
	dev := defaultOutputDevice()
	if dev == 0 {
		return 0, false
	}
	addr := volumeAddress()
	if !audioObjectHasProperty(dev, &addr) {
		return 0, false
	}
	var v float32
	size := uint32(4)
	if audioObjectGetPropertyData(dev, &addr, 0, 0, &size, unsafe.Pointer(&v)) != 0 {
		return 0, false
	}
	return v, true
}

func setSystemVolume(v float32) {
	dev := defaultOutputDevice()
	addr := volumeAddress()
	var settable uint8
	if dev == 0 || !audioObjectHasProperty(dev, &addr) || audioObjectIsPropertySettable(dev, &addr, &settable) != 0 || settable == 0 {
		return
	}
	audioObjectSetPropertyData(dev, &addr, 0, 0, 4, unsafe.Pointer(&v))
}

type audioPropertyAddress struct{ selector, scope, element uint32 }

// volumeAddress is kAudioHardwareServiceDeviceProperty_VirtualMainVolume
// ('vmvc') on the output scope ('outp'), the system volume slider's value.
func volumeAddress() audioPropertyAddress {
	return audioPropertyAddress{0x766D7663, 0x6F757470, 0}
}

func defaultOutputDevice() uint32 {
	// kAudioHardwarePropertyDefaultOutputDevice ('dOut') on the global
	// scope ('glob') of the system object (1).
	addr := audioPropertyAddress{0x644F7574, 0x676C6F62, 0}
	var dev uint32
	size := uint32(4)
	if audioObjectGetPropertyData(1, &addr, 0, 0, &size, unsafe.Pointer(&dev)) != 0 {
		return 0
	}
	return dev
}

var (
	loadOnce                                    sync.Once
	appServices, coreFoundation, carbon, libSys uintptr
	mainQueue                                   uintptr

	isSecureEventInputEnabled       func() bool
	cfRelease                       func(uintptr)
	cgEventTapCreate                func(tap, place, options uint32, mask uint64, callback, userInfo uintptr) uintptr
	cgEventTapEnable                func(tap uintptr, enable bool)
	cgEventTapIsEnabled             func(tap uintptr) bool
	cgEventGetIntegerValueField     func(event uintptr, field uint32) int64
	cgEventGetFlags                 func(event uintptr) uint64
	cgEventKeyboardGetUnicodeString func(event uintptr, max uint64, n *uint64, buf *uint16)
	cfMachPortCreateRunLoopSource   func(alloc, port uintptr, order int) uintptr
	cfMachPortInvalidate            func(port uintptr)
	cfRunLoopGetMain                func() uintptr
	cfRunLoopAddSource              func(loop, source, mode uintptr)
	cfRunLoopRemoveSource           func(loop, source, mode uintptr)
	cgMainDisplayID                 func() uint32
	cgDisplayHideCursor             func(display uint32) int32
	cgDisplayShowCursor             func(display uint32) int32
	cgWarpMouseCursorPosition       func(p cgPoint) int32
	objcAutoreleasePoolPush         func() uintptr
	objcAutoreleasePoolPop          func(uintptr)
	dispatchAsyncF                  func(queue, context, fn uintptr)
	audioObjectHasProperty          func(obj uint32, addr *audioPropertyAddress) bool
	audioObjectIsPropertySettable   func(obj uint32, addr *audioPropertyAddress, settable *uint8) int32
	audioObjectGetPropertyData      func(obj uint32, addr *audioPropertyAddress, qualSize uint32, qual uintptr, size *uint32, data unsafe.Pointer) int32
	audioObjectSetPropertyData      func(obj uint32, addr *audioPropertyAddress, qualSize uint32, qual uintptr, size uint32, data unsafe.Pointer) int32
)

func load() {
	loadOnce.Do(func() {
		open := func(path string) uintptr {
			h, err := purego.Dlopen(path, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
			if err != nil {
				panic(err)
			}
			return h
		}
		appServices = open("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices")
		coreFoundation = open("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
		carbon = open("/System/Library/Frameworks/Carbon.framework/Carbon")
		coreAudio := open("/System/Library/Frameworks/CoreAudio.framework/CoreAudio")
		libSys = open("/usr/lib/libSystem.B.dylib")
		libObjc := open("/usr/lib/libobjc.A.dylib")
		open("/System/Library/Frameworks/AppKit.framework/AppKit") // NSEvent, NSScreen

		purego.RegisterLibFunc(&isSecureEventInputEnabled, carbon, "IsSecureEventInputEnabled")
		purego.RegisterLibFunc(&cfRelease, coreFoundation, "CFRelease")
		purego.RegisterLibFunc(&cgEventTapCreate, appServices, "CGEventTapCreate")
		purego.RegisterLibFunc(&cgEventTapEnable, appServices, "CGEventTapEnable")
		purego.RegisterLibFunc(&cgEventTapIsEnabled, appServices, "CGEventTapIsEnabled")
		purego.RegisterLibFunc(&cgEventGetIntegerValueField, appServices, "CGEventGetIntegerValueField")
		purego.RegisterLibFunc(&cgEventGetFlags, appServices, "CGEventGetFlags")
		purego.RegisterLibFunc(&cgEventKeyboardGetUnicodeString, appServices, "CGEventKeyboardGetUnicodeString")
		purego.RegisterLibFunc(&cfMachPortCreateRunLoopSource, coreFoundation, "CFMachPortCreateRunLoopSource")
		purego.RegisterLibFunc(&cfMachPortInvalidate, coreFoundation, "CFMachPortInvalidate")
		purego.RegisterLibFunc(&cfRunLoopGetMain, coreFoundation, "CFRunLoopGetMain")
		purego.RegisterLibFunc(&cfRunLoopAddSource, coreFoundation, "CFRunLoopAddSource")
		purego.RegisterLibFunc(&cfRunLoopRemoveSource, coreFoundation, "CFRunLoopRemoveSource")
		purego.RegisterLibFunc(&cgMainDisplayID, appServices, "CGMainDisplayID")
		purego.RegisterLibFunc(&cgDisplayHideCursor, appServices, "CGDisplayHideCursor")
		purego.RegisterLibFunc(&cgDisplayShowCursor, appServices, "CGDisplayShowCursor")
		purego.RegisterLibFunc(&cgWarpMouseCursorPosition, appServices, "CGWarpMouseCursorPosition")
		purego.RegisterLibFunc(&objcAutoreleasePoolPush, libObjc, "objc_autoreleasePoolPush")
		purego.RegisterLibFunc(&objcAutoreleasePoolPop, libObjc, "objc_autoreleasePoolPop")
		purego.RegisterLibFunc(&dispatchAsyncF, libSys, "dispatch_async_f")
		purego.RegisterLibFunc(&audioObjectHasProperty, coreAudio, "AudioObjectHasProperty")
		purego.RegisterLibFunc(&audioObjectIsPropertySettable, coreAudio, "AudioObjectIsPropertySettable")
		purego.RegisterLibFunc(&audioObjectGetPropertyData, coreAudio, "AudioObjectGetPropertyData")
		purego.RegisterLibFunc(&audioObjectSetPropertyData, coreAudio, "AudioObjectSetPropertyData")
		mainQueue = sym(libSys, "_dispatch_main_q")
	})
}

// sym is the address of a library's exported variable.
func sym(lib uintptr, name string) uintptr {
	p, err := purego.Dlsym(lib, name)
	if err != nil {
		panic(err)
	}
	return p
}

// symValue is the value of a library's exported pointer variable, such as a
// CFStringRef constant.
func symValue(lib uintptr, name string) uintptr {
	p := sym(lib, name)
	return **(**uintptr)(unsafe.Pointer(&p))
}
