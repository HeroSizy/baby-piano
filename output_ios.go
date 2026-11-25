//go:build ios

package main

import (
	"log"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/audio"
	"github.com/nsgo-dev/nsgo/core"
)

// routePickerComponent is AVKit's AirPlay button, AVRoutePickerView: it
// opens the system's list of outputs, the iPhone's speaker, headphones,
// Bluetooth speakers and AirPlay, and moves the piano's sound to the one
// chosen.
const routePickerComponent = "routePicker"

func platformOptions() []nsgo.Option {
	keepSession()
	return []nsgo.Option{nsgo.Component(routePickerComponent, newRoutePicker)}
}

var sel = objc.RegisterName

// frameworks loads AVFAudio for the audio session and AVKit for the
// picker, and the autorelease pool's calls.
var frameworks = sync.OnceValue(func() bool {
	for _, path := range []string{
		"/System/Library/Frameworks/AVFAudio.framework/AVFAudio",
		"/System/Library/Frameworks/AVKit.framework/AVKit",
	} {
		if _, err := purego.Dlopen(path, purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			return false
		}
	}
	lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_LAZY)
	if err != nil {
		return false
	}
	purego.RegisterLibFunc(&poolPush, lib, "objc_autoreleasePoolPush")
	purego.RegisterLibFunc(&poolPop, lib, "objc_autoreleasePoolPop")
	return objc.GetClass("AVAudioSession") != 0
})

var (
	poolPush func() uintptr
	poolPop  func(uintptr)
)

func session() objc.ID {
	return objc.ID(objc.GetClass("AVAudioSession")).Send(sel("sharedInstance"))
}

// keepSession holds iOS's audio session for as long as the app runs, as a
// music app's: the Playback category, so the piano plays with the
// Ring/Silent switch on and moves with the system's output to headphones,
// a Bluetooth speaker or AirPlay as they connect.
//
// Sounds alone hold the session ambient only while they play, and NSGo
// deactivates it a second after the last; a route that changed meanwhile,
// headphones plugged in between two notes, left the audio engine on the
// old output. Held, the session stays active, and the engine hears of every
// route change and starts again on the new output.
//
// The system deactivates a held session too, for a call, an alarm or an
// app that takes the audio, and nothing activates it again, so the app does
// as the interruption ends and whenever it comes back to the front.
func keepSession() {
	if !frameworks() {
		return
	}
	if _, err := audio.HoldSession(core.AudioSessionPlayback); err != nil {
		log.Print("mash piano: cannot hold the audio session: ", err)
		return
	}
	observer, err := objc.RegisterClass("MashPianoSessionObserver", objc.GetClass("NSObject"), nil, nil,
		[]objc.MethodDef{
			{Cmd: sel("becameActive:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) { activate() }},
			{Cmd: sel("interrupted:"), Fn: func(_ objc.ID, _ objc.SEL, note objc.ID) {
				const ended = 0 // AVAudioSessionInterruptionTypeEnded
				kind := note.Send(sel("userInfo")).Send(sel("objectForKey:"), nsString("AVAudioSessionInterruptionTypeKey"))
				if kind != 0 && objc.Send[uint](kind, sel("unsignedIntegerValue")) == ended {
					activate()
				}
			}},
		})
	if err != nil {
		log.Print("mash piano: ", err)
		return
	}
	o := objc.ID(observer).Send(sel("new"))
	center := objc.ID(objc.GetClass("NSNotificationCenter")).Send(sel("defaultCenter"))
	// The notifications' names are their constants' values.
	center.Send(sel("addObserver:selector:name:object:"), o, sel("becameActive:"), nsString("UIApplicationDidBecomeActiveNotification"), objc.ID(0))
	center.Send(sel("addObserver:selector:name:object:"), o, sel("interrupted:"), nsString("AVAudioSessionInterruptionNotification"), objc.ID(0))
}

// activate activates the audio session again, in the category it holds.
func activate() {
	if !objc.Send[bool](session(), sel("setActive:error:"), true, objc.ID(0)) {
		log.Print("mash piano: cannot activate the audio session")
	}
}

// outputName is the name of the output the piano plays through, as the
// system's picker shows it: "iPhone", "AirPods Pro", a speaker's name.
func outputName() string {
	if !frameworks() {
		return ""
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := poolPush()
	defer poolPop(pool)
	port := session().Send(sel("currentRoute")).Send(sel("outputs")).Send(sel("firstObject"))
	if port == 0 {
		return ""
	}
	return goString(port.Send(sel("portName")))
}

// nsString is s as an autoreleased NSString.
func nsString(s string) objc.ID {
	b := append([]byte(s), 0)
	str := objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), unsafe.Pointer(&b[0]))
	runtime.KeepAlive(b)
	return str
}

func goString(str objc.ID) string {
	if str == 0 {
		return ""
	}
	p := objc.Send[unsafe.Pointer](str, sel("UTF8String"))
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(p, n)) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(p), n))
}

// routePicker is an AVRoutePickerView in the setup's colors.
type routePicker struct{ view objc.ID }

func newRoutePicker(cx *core.NativeContext) core.NativeComponent {
	if !frameworks() || objc.GetClass("AVRoutePickerView") == 0 {
		return nil
	}
	p := &routePicker{view: objc.ID(objc.GetClass("AVRoutePickerView")).Send(sel("new"))}
	p.tint(false)
	cx.SetSize(core.Size{Width: 44, Height: 44})
	return p
}

func (p *routePicker) View() any { return p.view }

// SetProp takes "dark", whether the setup shows its dark appearance.
func (p *routePicker) SetProp(name string, value any) {
	if dark, ok := value.(bool); ok && name == "dark" {
		p.tint(dark)
	}
}

// tint colors the button the page's ink, and coral while the sound plays
// somewhere other than the iPhone itself.
func (p *routePicker) tint(dark bool) {
	ink := lightTones.ink
	if dark {
		ink = darkTones.ink
	}
	p.view.Send(sel("setTintColor:"), uiColor(ink))
	p.view.Send(sel("setActiveTintColor:"), uiColor(coral))
}

func uiColor(c core.Color) objc.ID {
	r, g, b, a := c.Components()
	return objc.ID(objc.GetClass("UIColor")).Send(sel("colorWithRed:green:blue:alpha:"), r, g, b, a)
}

func (p *routePicker) Command(string, any) {}

func (p *routePicker) Close() { p.view.Send(sel("release")) }
