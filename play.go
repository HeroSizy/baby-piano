package main

import (
	"math/rand/v2"
	"slices"
	"time"

	"github.com/nsgo-dev/nsgo/core"

	"github.com/HeroSizy/baby-piano/locales"
)

// exitHold is how long parents hold the top-right corner to stop.
const (
	exitHold   = 3 * time.Second
	cornerSize = 90
)

// playScreen is the piano: on a computer a dark screen the keyboard plays,
// on a phone or tablet big colored keys, until a keyboard is attached.
func (a *app) playScreen(cx *core.Scope) core.Node {
	fx := newEffects(cx)
	if a.settings.visuals.Get() {
		a.burst = fx.burst
		cx.Cleanup(func() { a.burst = nil })
	}
	// A phone or tablet with a keyboard plays as a computer does, the
	// screen all the notes' own: the system says one is attached, or a
	// key pressed shows it, should the system not. Without one again,
	// the big keys come back.
	typed := core.NewSignal(cx, false)
	cx.Effect(func() {
		if !cx.Env().HardwareKeyboard() {
			typed.Set(false)
		}
	})
	touch := core.NewMemo(cx, func() bool {
		return !a.locked && !a.kiosk.Supported() && !cx.Env().HardwareKeyboard() && !typed.Get()
	})
	press := func(name string) {
		typed.Set(true)
		text := ""
		if len([]rune(name)) == 1 {
			text = name
		}
		a.key(name, text)
	}
	if !a.kiosk.Supported() {
		// No view of a phone's or tablet's takes the keyboard focus, so
		// it hears a keyboard as the keys held: each key that goes down
		// plays.
		var was []string
		cx.Effect(func() {
			now := core.KeysDown(cx)
			// Under the screen's scope, not this effect's: the notes'
			// fireworks belong to the screen, and the effect running
			// again as the key comes up would stop them halfway.
			cx.Untrack(func() {
				for _, k := range now {
					if !slices.Contains(was, k) {
						cx.Run(func() { press(k) })
					}
				}
			})
			was = now
		})
	}
	// Phones and tablets play with every system bar and gesture hint
	// hidden that the platform lets an app hide; desktops have none.
	cx.SetStatusBar(core.StatusBarSetting{Immersive: core.ToggleOn})
	cx.Cleanup(func() { a.cx.SetStatusBar(core.StatusBarSetting{Immersive: core.ToggleOff}) })
	// A locked mouse plays nothing and moves nothing: no pointer either.
	cursor := core.CursorAuto
	if a.locked && a.settings.lockMouse.Get() {
		cursor = core.CursorNone
	}
	root := &core.Ref{}
	cx.RequestAnimationFrame(func(time.Time) { root.Focus() })
	return core.View{
		TestID:    "piano",
		Ref:       root,
		Focusable: func() bool { return true },
		// Should the lock miss a key, as while another app holds secure
		// input, the window hears it here and plays it all the same.
		OnKeyDown: func(e core.KeyEvent) bool {
			if !e.Repeat {
				press(e.Key)
			}
			return true
		},
		// A click away from the corner plays a note from the middle of
		// the keyboard.
		OnPressDown: func(e core.PressEvent) {
			if touch.GetUntracked() || a.locked && a.settings.lockMouse.Get() {
				return // the keys play touches; a locked mouse plays nothing
			}
			i := 10 + rand.IntN(16) // C3 … C6
			if a.play(i) {
				fx.burstAt(i, e.PagePosition)
			}
		},
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, Background: core.RGB(0x000000), Overflow: core.Hidden, Cursor: cursor},
		Children: []core.Node{
			core.Show{When: touch.Get, Then: func(cx *core.Scope) core.Node { return a.touchKeys(cx, fx.burstAt) }},
			fx.node(),
			// A phone or tablet that turns to its keyboard says so for a
			// moment, so parents know why the big keys went; it fades, so
			// nothing stays over the fireworks.
			core.Show{When: func() bool { return !a.kiosk.Supported() && !touch.Get() }, Then: func(cx *core.Scope) core.Node {
				fade := core.NewAnimatedValue(cx, 1)
				cx.StartAnimation(core.Timing(fade, 0, core.TimingConfig{Duration: 800 * time.Millisecond, Delay: 2500 * time.Millisecond}), nil)
				return core.View{TestID: "keyboardOn",
					Style: core.Style{Position: core.Absolute, Left: core.Pt(16), Bottom: core.Pt(16), Padding: core.Symmetric(8, 14), CornerRadius: 999,
						Background: core.RGBA(0xffffff, 0.12), Animated: &core.AnimatedStyle{Opacity: fade}},
					Children: []core.Node{core.Text{Content: "⌨️  " + locales.KeyboardOn(cx),
						Style: core.Style{FontSize: 14, Color: core.RGBA(0xffffff, 0.8)}}}}
			}},
			a.corner(cx),
		},
	}
}

// corner is where parents stop the piano: holding the top-right corner for
// exitHold fills a ring and stops it, letting go sooner does nothing.
func (a *app) corner(cx *core.Scope) core.Node {
	if a.locked && a.settings.lockMouse.Get() {
		return nil // the mouse is locked: parents type the exit word
	}
	progress := core.NewAnimatedValue(cx, 0)
	var run *core.AnimationRun
	var stop func() bool
	cancel := func() {
		if stop != nil {
			stop()
			stop = nil
		}
		if run != nil {
			run.Stop()
		}
		progress.Set(0)
	}
	ring := core.Interpolate(progress, core.Interpolation{Input: []float64{0, 1}, Output: []float64{0.15, 1}})
	return core.View{
		Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0),
			Width: core.Pt(cornerSize), Height: core.Pt(cornerSize), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
		Label: locales.CornerHint(cx), Role: core.RoleButton, NoFocus: true, TestID: "corner",
		OnPressDown: func(core.PressEvent) {
			cancel()
			run = cx.StartAnimation(core.Timing(progress, 1, core.TimingConfig{Duration: exitHold}), nil)
			stop = cx.AfterFunc(exitHold, func() { stop = nil; a.leave() })
		},
		OnPressUp: func(core.PressEvent) { cancel() },
		Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(40), Height: core.Pt(40), CornerRadius: 20,
				BorderWidth: 4, BorderColor: core.RGBA(0xffffff, 0.85),
				Animated: &core.AnimatedStyle{Opacity: ring, Scale: ring}}},
			core.Text{Content: locales.CornerHint(cx), Style: core.Style{Position: core.Absolute, Top: core.Pt(cornerSize - 6), Right: core.Pt(12),
				Width: core.Pt(260), TextAlign: core.TextAlignRight, FontSize: 11, Color: core.RGBA(0xffffff, 0.3)}},
		},
	}
}
