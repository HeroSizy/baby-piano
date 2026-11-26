package main

import (
	"math/rand/v2"
	"slices"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/HeroSizy/baby-piano/locales"
)

// exitHold is how long parents hold the top-right corner to stop.
const (
	exitHold   = 3 * time.Second
	cornerSize = 90
)

// playScreen is the piano: on a computer a quiet screen the keyboard
// plays, on a phone or tablet big colored keys, until a keyboard is
// attached. It is the setup's warm paper in light appearance and its plum
// night in dark, following the system's as it changes. Every note shows:
// as a firework, or with the fireworks off as a soft bloom of its color,
// and on the keys as the key lighting up.
func (a *app) playScreen(cx *core.Scope) core.Node {
	visuals := a.settings.visuals.Get()
	fx := newEffects(cx, visuals)
	a.burst = fx.burst
	cx.Cleanup(func() { a.burst = nil })
	// The keys light up themselves; fireworks go over them only when asked.
	keyBurst := func(int, core.Point) {}
	if visuals {
		keyBurst = fx.burstAt
	}
	// A phone or tablet with a keyboard plays as a computer does, the
	// screen all the notes' own: the system says one is attached, or a
	// key pressed shows it, should the system not. Without one again,
	// the big keys come back.
	typed := core.NewSignal(cx, false)
	cx.Effect(func() {
		if !a.keyboard.Get() {
			typed.Set(false)
		}
	})
	touch := core.NewMemo(cx, func() bool {
		return !a.locked && !a.kiosk.Supported() && !a.keyboard.Get() && !typed.Get()
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
		StyleFunc: func() core.Style {
			t := tonesOf(cx)
			return core.Style{FlexGrow: 1, FlexShrink: 1, Overflow: core.Hidden, Cursor: cursor,
				Background: t.paperTop, Gradients: []core.Gradient{core.LinearGradient(180, t.paperTop, t.paperBottom)}}
		},
		Children: []core.Node{
			core.Show{When: touch.Get, Then: func(cx *core.Scope) core.Node { return a.touchKeys(cx, keyBurst) }},
			fx.node(),
			// A phone or tablet that turns to its keyboard says so for a
			// moment, so parents know why the big keys went; it fades, so
			// nothing stays over the fireworks.
			core.Show{When: func() bool { return !a.kiosk.Supported() && !touch.Get() }, Then: func(cx *core.Scope) core.Node {
				fade := core.NewAnimatedValue(cx, 1)
				cx.StartAnimation(core.Timing(fade, 0, core.TimingConfig{Duration: 800 * time.Millisecond, Delay: 2500 * time.Millisecond}), nil)
				ink := func() core.Color { return tonesOf(cx).ink.Faded(0.8) }
				return core.View{TestID: "keyboardOn",
					StyleFunc: func() core.Style {
						return core.Style{Position: core.Absolute, Left: core.Pt(16), Bottom: core.Pt(16), Padding: core.Symmetric(8, 14), CornerRadius: 999,
							Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8,
							Background: tonesOf(cx).ink.Faded(0.1), Animated: &core.AnimatedStyle{Opacity: fade}}
					},
					Children: []core.Node{
						core.Icon{SVG: iconKeyboard, StyleFunc: func() core.Style { return core.Style{Width: core.Pt(18), Height: core.Pt(18), Color: ink()} }},
						core.Text{Content: locales.KeyboardOn(cx), StyleFunc: func() core.Style { return core.Style{FontSize: 14, Color: ink()} }}}}
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
	hint := core.NewAnimatedValue(cx, 1)
	cx.StartAnimation(core.Timing(hint, 0, core.TimingConfig{Duration: time.Second, Delay: 5 * time.Second}), nil)
	ring := core.Interpolate(progress, core.Interpolation{Input: []float64{0, 1}, Output: []float64{0.15, 1}})
	return core.View{
		// Clear of a camera cutout, as the keys are, so parents see the
		// ring fill: the corner sits at the keys' top-right, not the glass's.
		Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0),
			MarginEnv: core.EnvEdges{Top: core.EnvSafeArea, Right: core.EnvSafeArea},
			Width:     core.Pt(cornerSize), Height: core.Pt(cornerSize), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
		Label: locales.CornerHint(cx), Role: core.RoleButton, NoFocus: true, TestID: "corner",
		OnPressDown: func(core.PressEvent) {
			cancel()
			run = cx.StartAnimation(core.Timing(progress, 1, core.TimingConfig{Duration: exitHold}), nil)
			stop = cx.AfterFunc(exitHold, func() { stop = nil; a.leave() })
		},
		OnPressUp: func(core.PressEvent) { cancel() },
		Children: []core.Node{
			// The ring in the screen's ink: white on the night, plum on the paper.
			core.View{StyleFunc: func() core.Style {
				return core.Style{Width: core.Pt(40), Height: core.Pt(40), CornerRadius: 20,
					BorderWidth: 4, BorderColor: tonesOf(cx).ink.Faded(0.85),
					Animated: &core.AnimatedStyle{Opacity: ring, Scale: ring}}
			}},
			// The hint is a little card like the setup page's tips, the
			// waving hand beside the words, with a tail pointing up at the
			// ring; it reads over any key color, keeps clear of the screen
			// edge, and fades once parents have seen it.
			core.View{PointerEvents: core.PointerEventsNone,
				Style: core.Style{Position: core.Absolute, Top: core.Pt(cornerSize - 14), Right: core.Pt(12),
					Width: core.Pt(300), AlignItems: core.AlignEnd, // the corner itself is narrow
					Animated: &core.AnimatedStyle{Opacity: hint}},
				Children: []core.Node{
					core.View{StyleFunc: func() core.Style {
						return core.Style{Width: core.Pt(16), Height: core.Pt(16), CornerRadius: 4, Background: tonesOf(cx).card,
							Margin: core.Edges{Right: cornerSize/2 - 12 - 8, Bottom: -10}, Transform: core.Transform{Rotate: 45}}
					}},
					core.View{StyleFunc: func() core.Style {
						t := tonesOf(cx)
						return core.Style{MaxWidth: core.Pt(300), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Edges{Top: 10, Bottom: 10, Left: 10, Right: 16},
							CornerRadius: 20, Background: t.card, Shadows: []core.BoxShadow{{Offset: core.Point{Y: 8}, Blur: 24, Color: t.cardShadow}}}
					}, Children: []core.Node{
						badge(cx, iconWave, accentOrange, 32, 20, 10),
						core.Text{Content: locales.CornerHint(cx), StyleFunc: inked(cx, core.Style{FontSize: 14, LineHeight: 19, FontWeight: ui.Semibold, FlexShrink: 1})},
					}},
				}},
		},
	}
}
