package main

import (
	"math/rand/v2"
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
// on a phone or tablet big colored keys.
func (a *app) playScreen(cx *core.Scope) core.Node {
	fx := newEffects(cx)
	if a.settings.visuals.Get() {
		a.burst = fx.burst
		cx.Cleanup(func() { a.burst = nil })
	}
	var keys core.Node
	if !a.locked && !a.kiosk.Supported() {
		keys = a.touchKeys(cx, fx.burstAt)
	}
	root := &core.Ref{}
	cx.RequestAnimationFrame(func(time.Time) { root.Focus() })
	return core.View{
		Ref:       root,
		Focusable: func() bool { return true },
		// Should the lock miss a key, as while another app holds secure
		// input, the window hears it here and plays it all the same.
		OnKeyDown: func(e core.KeyEvent) bool {
			if !e.Repeat {
				text := ""
				if len([]rune(e.Key)) == 1 {
					text = e.Key
				}
				a.key(e.Key, text)
			}
			return true
		},
		// A click away from the corner plays a note from the middle of
		// the keyboard.
		OnPressDown: func(e core.PressEvent) {
			if keys != nil || a.locked && a.settings.lockMouse.Get() {
				return // the keys play touches; a locked mouse plays nothing
			}
			i := 10 + rand.IntN(16) // C3 … C6
			if a.play(i) {
				fx.burstAt(i, e.PagePosition)
			}
		},
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, Background: core.RGB(0x000000), Overflow: core.Hidden},
		Children: []core.Node{
			keys,
			fx.node(),
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
