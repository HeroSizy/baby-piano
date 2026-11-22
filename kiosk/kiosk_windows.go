//go:build windows

package kiosk

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/HeroSizy/baby-piano/piano"
)

// Supported reports whether this platform locks the keyboard.
func Supported() bool { return true }

// SecureInput is false: Windows has no secure keyboard entry that a
// low-level hook cannot hear.
func SecureInput() bool { return false }

// ErrNotAllowed is Start's error when the system refused the keyboard hook.
var ErrNotAllowed = errors.New("kiosk: Windows refused the keyboard hook")

// The lock runs on a thread of its own: low-level hooks call back on the
// thread that set them, which must pump messages, and the covering windows
// and the calls that bring the piano to the front live there too. Other
// functions post it commands.
var lock struct {
	sync.Mutex
	thread    uint32        // 0 while unlocked
	done      chan struct{} // closed once the thread restored the system
	lockMouse bool
	onKey     func(Key)
	down      map[uint32]bool // keys held, so a held key plays once
	keyboard  uintptr
	mouse     uintptr
	main      windows.HWND   // the piano's window, once Enter found it
	covers    []windows.HWND // black windows over the other screens
}

// Commands posted to the lock's thread.
const (
	wmApp    = 0x8000
	cmdEnter = wmApp + 1
	cmdFront = wmApp + 2
	cmdRearm = wmApp + 3
)

// Start swallows every key, the Windows key, Alt+Tab, Alt+F4 and the media
// keys among them, and with opts.LockMouse the mouse and touchpad too,
// passing each key pressed (not repeated) to onKey on a goroutine of its
// own. It also turns off the accessibility hotkeys (Shift five times and the
// like) and Win+L, restoring them on Stop. Ctrl+Alt+Del stays: no app can
// take it. It runs until Stop.
func Start(opts Options, onKey func(Key)) error {
	lock.Lock()
	if lock.thread != 0 {
		lock.Unlock()
		return nil
	}
	lock.lockMouse, lock.onKey, lock.down = opts.LockMouse, onKey, map[uint32]bool{}
	done := make(chan struct{})
	lock.done = done
	lock.Unlock()

	started := make(chan error, 1)
	go func() {
		runtime.LockOSThread() // the hooks and windows are this thread's
		defer runtime.UnlockOSThread()
		defer close(done)
		var msg winMsg
		peekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 0) // make the thread's message queue
		lock.Lock()
		ok := arm()
		if ok {
			lock.thread = windows.GetCurrentThreadId()
		}
		lock.Unlock()
		if !ok {
			started <- ErrNotAllowed
			return
		}
		guardSystem()
		started <- nil
		for {
			r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(r) <= 0 { // WM_QUIT, or an error
				break
			}
			switch msg.message {
			case cmdEnter:
				enter()
			case cmdFront:
				front()
			case cmdRearm:
				lock.Lock()
				arm()
				lock.Unlock()
			default:
				translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
				dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
			}
		}
		lock.Lock()
		for _, h := range []uintptr{lock.keyboard, lock.mouse} {
			if h != 0 {
				unhookWindowsHookEx.Call(h)
			}
		}
		for _, w := range lock.covers {
			destroyWindow.Call(uintptr(w))
		}
		if lock.main != 0 {
			const hwndNoTopmost = ^uintptr(1) // -2
			const noSize, noMove, noActivate = 0x1, 0x2, 0x10
			setWindowPos.Call(uintptr(lock.main), hwndNoTopmost, 0, 0, 0, 0, noSize|noMove|noActivate)
		}
		lock.keyboard, lock.mouse, lock.covers, lock.main, lock.onKey = 0, 0, nil, 0, nil
		lock.Unlock()
		restoreSystem()
	}()
	return <-started
}

// arm sets the hooks, new ones before the old ones go, so no key slips
// between: Windows silently drops a hook that once answered too slowly, and
// arming again every tick puts it back. Call it on the lock's thread, with
// lock held.
func arm() bool {
	set := func(kind uintptr, proc uintptr, old uintptr) uintptr {
		h, _, _ := setWindowsHookEx.Call(kind, proc, moduleHandle(), 0)
		if h == 0 {
			return old
		}
		if old != 0 {
			unhookWindowsHookEx.Call(old)
		}
		return h
	}
	const keyboardLL, mouseLL = 13, 14
	lock.keyboard = set(keyboardLL, keyboardProc, lock.keyboard)
	if lock.lockMouse {
		lock.mouse = set(mouseLL, mouseProc, lock.mouse)
	}
	return lock.keyboard != 0
}

// Stop gives the keyboard back and restores what Start turned off, before
// it returns, so the app may quit straight after.
func Stop() {
	lock.Lock()
	thread, done := lock.thread, lock.done
	lock.thread = 0
	lock.Unlock()
	if thread == 0 {
		return
	}
	const quit = 0x0012
	postThreadMessage.Call(uintptr(thread), quit, 0, 0)
	<-done
}

// post sends a command to the lock's thread, if it runs.
func post(cmd uint32) {
	lock.Lock()
	defer lock.Unlock()
	if lock.thread != 0 {
		postThreadMessage.Call(uintptr(lock.thread), uintptr(cmd), 0, 0)
	}
}

type kbdllHook struct {
	vkCode, scanCode, flags, time uint32
	extraInfo                     uintptr
}

var keyboardProc = windows.NewCallback(func(code, wParam, lParam uintptr) uintptr {
	if int32(code) < 0 {
		r, _, _ := callNextHookEx.Call(0, code, wParam, lParam)
		return r
	}
	k := *(**kbdllHook)(unsafe.Pointer(&lParam)) // the system owns it for the call
	const keyDown, keyUp, sysKeyDown, sysKeyUp = 0x100, 0x101, 0x104, 0x105
	lock.Lock()
	var onKey func(Key)
	switch wParam {
	case keyDown, sysKeyDown:
		if !lock.down[k.vkCode] {
			lock.down[k.vkCode] = true
			onKey = lock.onKey
		}
	case keyUp, sysKeyUp:
		delete(lock.down, k.vkCode)
	}
	lock.Unlock()
	if onKey != nil {
		const extended = 0x01
		key := Key{Name: piano.WindowsKeyName(int(k.scanCode), k.flags&extended != 0)}
		if k.vkCode >= 'A' && k.vkCode <= 'Z' { // the exit word's letters
			key.Text = string(rune(k.vkCode - 'A' + 'a'))
		}
		go onKey(key)
	}
	return 1 // swallowed: neither the system nor any app hears it
})

var mouseProc = windows.NewCallback(func(code, wParam, lParam uintptr) uintptr {
	if int32(code) < 0 {
		r, _, _ := callNextHookEx.Call(0, code, wParam, lParam)
		return r
	}
	return 1 // the pointer stays put and clicks do nothing
})

// Tick keeps the lock and the volume cap in force; call it every half
// second while playing. It arms the hooks again and lowers the volume past
// MaxSystemVolume, and reports whether the output device changed since the
// last tick, as a Bluetooth speaker connecting, so the app can hold its
// notes a moment.
func Tick() (outputChanged bool) {
	post(cmdRearm)
	var changed bool
	withEndpoint(func(id string, vol *comObject) {
		changed = lastDevice != "" && id != lastDevice
		lastDevice = id
		// Step down rather than set the level: setting it takes a float
		// argument, which a system call cannot pass on every architecture.
		const getScalar, stepDown = 9, 16
		for range 100 {
			var v float32
			if vol.call(getScalar, uintptr(unsafe.Pointer(&v))) != 0 || v <= MaxSystemVolume {
				return
			}
			if vol.call(stepDown, uintptr(unsafe.Pointer(&windows.GUID{}))) != 0 {
				return
			}
		}
	})
	return changed
}

var lastDevice string

// SystemVolume reads the default output device's volume, from 0 to 1.
func SystemVolume() (v float32, ok bool) {
	withEndpoint(func(_ string, vol *comObject) {
		const getScalar = 9
		ok = vol.call(getScalar, uintptr(unsafe.Pointer(&v))) == 0
	})
	return v, ok
}

// Enter covers every screen: the piano's window stays on top of every
// other, full screen as the app made it, and the other screens get black
// windows, until Stop.
func Enter() { post(cmdEnter) }

// Front brings the piano back in front if the system or another app took it.
func Front() { post(cmdFront) }

// enter runs on the lock's thread.
func enter() {
	main := ownWindow()
	if main == 0 {
		return
	}
	lock.Lock()
	lock.main = main
	lock.Unlock()
	topmost(main)
	const defaultToNearest = 2
	mainMonitor, _, _ := monitorFromWindow.Call(uintptr(main), defaultToNearest)
	var covers []windows.HWND
	enumDisplayMonitors.Call(0, 0, windows.NewCallback(func(monitor, _, rect, _ uintptr) uintptr {
		if monitor != mainMonitor {
			if w := cover(*(**windows.Rect)(unsafe.Pointer(&rect))); w != 0 {
				covers = append(covers, w)
			}
		}
		return 1
	}), 0)
	lock.Lock()
	lock.covers = append(lock.covers, covers...)
	lock.Unlock()
}

// front runs on the lock's thread.
func front() {
	lock.Lock()
	main, covers := lock.main, lock.covers
	lock.Unlock()
	if main == 0 {
		return
	}
	for _, w := range covers {
		topmost(w)
	}
	fg, _, _ := getForegroundWindow.Call()
	if windows.HWND(fg) == main {
		return
	}
	// Windows lets only the foreground's input thread hand it over, so the
	// lock's thread joins it for the moment.
	fgThread, _, _ := getWindowThreadProcessId.Call(fg, 0)
	self := uintptr(windows.GetCurrentThreadId())
	if fgThread != 0 && fgThread != self {
		attachThreadInput.Call(self, fgThread, 1)
		defer attachThreadInput.Call(self, fgThread, 0)
	}
	const restore = 9
	if iconic, _, _ := isIconic.Call(uintptr(main)); iconic != 0 {
		showWindow.Call(uintptr(main), restore)
	}
	topmost(main)
	setForegroundWindow.Call(uintptr(main))
}

func topmost(w windows.HWND) {
	const hwndTopmost = ^uintptr(0) // -1
	const noSize, noMove, noActivate = 0x1, 0x2, 0x10
	setWindowPos.Call(uintptr(w), hwndTopmost, 0, 0, 0, 0, noSize|noMove|noActivate)
}

// ownWindow is the app's visible top-level window that NSGo opened.
func ownWindow() windows.HWND {
	pid := windows.GetCurrentProcessId()
	var found windows.HWND
	enumWindows.Call(windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		var owner uint32
		getWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
		if visible, _, _ := isWindowVisible.Call(hwnd); owner == pid && visible != 0 {
			found = windows.HWND(hwnd)
			return 0
		}
		return 1
	}), 0)
	return found
}

// cover opens a black window over r.
func cover(r *windows.Rect) windows.HWND {
	coverClassOnce.Do(func() {
		name, _ := windows.UTF16PtrFromString("BabyPianoCover")
		const blackBrush = 4
		brush, _, _ := getStockObject.Call(blackBrush)
		class := wndClassEx{
			wndProc:    defWindowProc.Addr(),
			instance:   moduleHandle(),
			background: brush,
			className:  name,
		}
		class.size = uint32(unsafe.Sizeof(class))
		if atom, _, _ := registerClassEx.Call(uintptr(unsafe.Pointer(&class))); atom != 0 {
			coverClass = name
		}
	})
	if coverClass == nil {
		return 0
	}
	const topmostEx, toolWindow = 0x8, 0x80
	const popup, visible = 0x80000000, 0x10000000
	w, _, _ := createWindowEx.Call(topmostEx|toolWindow, uintptr(unsafe.Pointer(coverClass)), 0, popup|visible,
		uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), 0, 0, moduleHandle(), 0)
	return windows.HWND(w)
}

var (
	coverClassOnce sync.Once
	coverClass     *uint16
)

type wndClassEx struct {
	size, style                        uint32
	wndProc                            uintptr
	clsExtra, wndExtra                 int32
	instance, icon, cursor, background uintptr
	menuName, className                *uint16
	iconSmall                          uintptr
}

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
	private uint32
}

func moduleHandle() uintptr {
	var h windows.Handle
	_ = windows.GetModuleHandleEx(0, nil, &h)
	return uintptr(h)
}

// The system's own state that Start changes and Stop restores.

// accessibility are the hotkeys parents rarely use and babies find: Shift
// five times (sticky keys), Num Lock held (toggle keys) and right Shift held
// (filter keys). Each is turned off for this sign-in only, unless the
// feature is on, and restored after.
var accessibility = []struct {
	get, set, size uint32
	saved          [6]uint32 // the SPI structure: cbSize, dwFlags, …
	changed        bool
}{
	{get: 0x3A, set: 0x3B, size: 8},  // STICKYKEYS
	{get: 0x34, set: 0x35, size: 8},  // TOGGLEKEYS
	{get: 0x32, set: 0x33, size: 24}, // FILTERKEYS
}

func guardSystem() {
	const on, hotkey, confirm = 0x1, 0x4, 0x8
	for i := range accessibility {
		a := &accessibility[i]
		a.saved = [6]uint32{a.size}
		if r, _, _ := systemParametersInfo.Call(uintptr(a.get), uintptr(a.size), uintptr(unsafe.Pointer(&a.saved)), 0); r == 0 {
			continue
		}
		if a.saved[1]&on != 0 {
			continue // in use: leave it be
		}
		s := a.saved
		s[1] &^= hotkey | confirm
		r, _, _ := systemParametersInfo.Call(uintptr(a.set), uintptr(a.size), uintptr(unsafe.Pointer(&s)), 0)
		a.changed = r != 0
	}
	blockWinL()
}

func restoreSystem() {
	for i := range accessibility {
		a := &accessibility[i]
		if a.changed {
			systemParametersInfo.Call(uintptr(a.set), uintptr(a.size), uintptr(unsafe.Pointer(&a.saved)), 0)
			a.changed = false
		}
	}
	restoreWinL()
}

// Win+L locks the computer before any hook hears it, and a baby's keys then
// go into the password box. The user's own policy DisableLockWorkstation
// turns it off; the value it had waits in a file until restored, so a
// crashed piano gives Win+L back on its next start.
const (
	policyKey   = `Software\Microsoft\Windows\CurrentVersion\Policies\System`
	policyValue = "DisableLockWorkstation"
)

func pendingFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "BabyPiano", "lock-policy-restore")
}

func blockWinL() {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, policyKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	old := "absent"
	if v, _, err := k.GetIntegerValue(policyValue); err == nil {
		if v == 1 {
			return // already off, perhaps by the workplace: not ours to touch
		}
		old = strconv.FormatUint(v, 10)
	}
	file := pendingFile()
	if file == "" || os.MkdirAll(filepath.Dir(file), 0o700) != nil || os.WriteFile(file, []byte(old), 0o600) != nil {
		return // could not remember the old value: leave Win+L alone
	}
	if k.SetDWordValue(policyValue, 1) != nil {
		os.Remove(file)
	}
}

func restoreWinL() {
	file := pendingFile()
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, policyKey, registry.SET_VALUE)
	if err != nil {
		return // keep the file and try again next start
	}
	defer k.Close()
	switch old := strings.TrimSpace(string(data)); old {
	case "absent":
		err = k.DeleteValue(policyValue)
		if errors.Is(err, registry.ErrNotExist) {
			err = nil
		}
	default:
		v, perr := strconv.ParseUint(old, 10, 32)
		if perr != nil {
			v = 0
		}
		err = k.SetDWordValue(policyValue, uint32(v))
	}
	if err == nil {
		os.Remove(file)
	}
}

// A piano that crashed while playing left Win+L off; give it back.
func init() { restoreWinL() }

// The system volume, through Core Audio's IAudioEndpointVolume.

type comObject struct{ vtbl *[32]uintptr }

func (o *comObject) call(method int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[method], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}

func (o *comObject) release() { o.call(2) }

// withEndpoint calls fn with the default output device's id and volume
// control, if there is one.
func withEndpoint(fn func(id string, volume *comObject)) {
	runtime.LockOSThread() // COM's apartment is the thread's
	defer runtime.UnlockOSThread()
	const multithreaded = 0x0
	if hr, _, _ := coInitializeEx.Call(0, multithreaded); int32(hr) >= 0 {
		defer coUninitialize.Call()
	} // else the thread already has an apartment, which serves as well

	clsid := windows.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidEnumerator := windows.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidVolume := windows.GUID{Data1: 0x5CDF2C82, Data2: 0x841E, Data3: 0x4546, Data4: [8]byte{0x97, 0x22, 0x0C, 0xF7, 0x40, 0x78, 0x22, 0x9A}}
	const clsctxAll = 0x17

	var enumerator *comObject
	if hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsid)), 0, clsctxAll, uintptr(unsafe.Pointer(&iidEnumerator)), uintptr(unsafe.Pointer(&enumerator))); hr != 0 {
		return
	}
	defer enumerator.release()
	const getDefaultEndpoint, render, multimedia = 4, 0, 1
	var device *comObject
	if enumerator.call(getDefaultEndpoint, render, multimedia, uintptr(unsafe.Pointer(&device))) != 0 || device == nil {
		return
	}
	defer device.release()
	const activate, getID = 3, 5
	var idPtr *uint16
	if device.call(getID, uintptr(unsafe.Pointer(&idPtr))) != 0 {
		return
	}
	id := windows.UTF16PtrToString(idPtr)
	windows.CoTaskMemFree(unsafe.Pointer(idPtr))
	var volume *comObject
	if device.call(activate, uintptr(unsafe.Pointer(&iidVolume)), clsctxAll, 0, uintptr(unsafe.Pointer(&volume))) != 0 || volume == nil {
		return
	}
	defer volume.release()
	fn(id, volume)
}

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")
	ole32  = windows.NewLazySystemDLL("ole32.dll")

	setWindowsHookEx         = user32.NewProc("SetWindowsHookExW")
	unhookWindowsHookEx      = user32.NewProc("UnhookWindowsHookEx")
	callNextHookEx           = user32.NewProc("CallNextHookEx")
	getMessage               = user32.NewProc("GetMessageW")
	peekMessage              = user32.NewProc("PeekMessageW")
	translateMessage         = user32.NewProc("TranslateMessage")
	dispatchMessage          = user32.NewProc("DispatchMessageW")
	postThreadMessage        = user32.NewProc("PostThreadMessageW")
	enumWindows              = user32.NewProc("EnumWindows")
	enumDisplayMonitors      = user32.NewProc("EnumDisplayMonitors")
	monitorFromWindow        = user32.NewProc("MonitorFromWindow")
	getWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	isWindowVisible          = user32.NewProc("IsWindowVisible")
	isIconic                 = user32.NewProc("IsIconic")
	showWindow               = user32.NewProc("ShowWindow")
	setWindowPos             = user32.NewProc("SetWindowPos")
	getForegroundWindow      = user32.NewProc("GetForegroundWindow")
	setForegroundWindow      = user32.NewProc("SetForegroundWindow")
	attachThreadInput        = user32.NewProc("AttachThreadInput")
	registerClassEx          = user32.NewProc("RegisterClassExW")
	createWindowEx           = user32.NewProc("CreateWindowExW")
	destroyWindow            = user32.NewProc("DestroyWindow")
	defWindowProc            = user32.NewProc("DefWindowProcW")
	systemParametersInfo     = user32.NewProc("SystemParametersInfoW")
	getStockObject           = gdi32.NewProc("GetStockObject")
	coInitializeEx           = ole32.NewProc("CoInitializeEx")
	coUninitialize           = ole32.NewProc("CoUninitialize")
	coCreateInstance         = ole32.NewProc("CoCreateInstance")
)
