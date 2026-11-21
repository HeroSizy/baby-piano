package main

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/nsgo-dev/nsgo/core"

	"github.com/HeroSizy/baby-piano/locales"
	"github.com/HeroSizy/baby-piano/piano"
)

// exitHold is how long parents hold the top-right corner to stop.
const (
	exitHold   = 3 * time.Second
	cornerSize = 90
)

// playScreen is the piano: on a computer a dark screen the keyboard plays,
// on a phone or tablet big colored keys.
func (a *app) playScreen(cx *core.Scope) core.Node {
	circles := newCircles(cx)
	if a.settings.visuals.Get() {
		a.burst = circles.burst
		cx.Cleanup(func() { a.burst = nil })
	}
	background := core.RGB(0x000000)
	if a.settings.visuals.Get() {
		background = core.RGB(0x121429)
	}
	var keys core.Node
	if !a.locked && !a.kiosk.Supported() {
		keys = a.touchKeys(cx)
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
				circles.burstAt(i, e.PagePosition)
			}
		},
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, Background: background, Overflow: core.Hidden},
		Children: []core.Node{
			keys,
			circles.node(),
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

// circles are a fixed few colored circles, each reused for the next note:
// it grows and fades over 1.8 seconds where the note's place on the scale
// puts it.
type circles struct {
	cx    *core.Scope
	slots []circle
	next  int
	size  *core.Signal[core.Size]
}

type circle struct {
	at      *core.Signal[core.Point]
	diam    *core.Signal[float64]
	color   *core.Signal[core.Color]
	opacity *core.AnimatedValue
	scale   *core.AnimatedValue
}

func newCircles(cx *core.Scope) *circles {
	c := &circles{cx: cx, size: core.NewSignal(cx, core.Size{})}
	for range 12 {
		c.slots = append(c.slots, circle{
			at:      core.NewSignal(cx, core.Point{}),
			diam:    core.NewSignal(cx, 200.0),
			color:   core.NewSignal(cx, core.RGB(0xffffff)),
			opacity: core.NewAnimatedValue(cx, 0),
			scale:   core.NewAnimatedValue(cx, 0.25),
		})
	}
	return c
}

// burst draws note i's circle across the screen by its pitch, low on the
// left.
func (c *circles) burst(i int) {
	s := c.size.GetUntracked()
	t := float64(i) / float64(len(piano.Notes)-1)
	x := float64(s.Width)*(0.08+0.84*t) + rand.Float64()*100 - 50
	y := float64(s.Height) * (0.15 + 0.7*rand.Float64())
	c.burstAt(i, core.Point{X: core.Points(x), Y: core.Points(y)})
}

// burstAt draws note i's circle centred at p.
func (c *circles) burstAt(i int, p core.Point) {
	slot := c.slots[c.next]
	c.next = (c.next + 1) % len(c.slots)
	t := float64(i) / float64(len(piano.Notes)-1)
	slot.color.Set(hsl(t*0.85*360, 0.9, 0.7))
	slot.diam.Set(150 + rand.Float64()*130)
	slot.at.Set(p)
	slot.opacity.Set(0)
	slot.scale.Set(0.25)
	grow := core.Timing(slot.scale, 1.35, core.TimingConfig{Duration: 1800 * time.Millisecond, Easing: core.BezierEasing(core.EaseOut)})
	fade := core.Sequence(
		core.Timing(slot.opacity, 0.85, core.TimingConfig{Duration: 270 * time.Millisecond}),
		core.Timing(slot.opacity, 0, core.TimingConfig{Duration: 1530 * time.Millisecond, Easing: core.BezierEasing(core.EaseOut)}),
	)
	c.cx.StartAnimation(core.Parallel(grow, fade), nil)
}

func (c *circles) node() core.Node {
	var nodes []core.Node
	for _, s := range c.slots {
		nodes = append(nodes, core.View{
			StyleFunc: func() core.Style {
				d := s.diam.Get()
				p := s.at.Get()
				return core.Style{Position: core.Absolute,
					Left: core.Pt(p.X - core.Points(d/2)), Top: core.Pt(p.Y - core.Points(d/2)),
					Width: core.Pt(core.Points(d)), Height: core.Pt(core.Points(d)), CornerRadius: core.Points(d / 2),
					Background: s.color.Get(), Animated: &core.AnimatedStyle{Opacity: s.opacity, Scale: s.scale}}
			},
		})
	}
	return core.View{
		Style:    core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), PointerEvents: core.PointerEventsNone},
		OnLayout: func(b core.Bounds) { c.size.Set(b.Size) },
		Children: nodes,
	}
}

// hsl is the color of hue h (degrees), saturation s and lightness l (0 to 1).
func hsl(h, s, l float64) core.Color {
	f := func(n float64) uint32 {
		k := math.Mod(n+h/30, 12)
		a := s * min(l, 1-l)
		return uint32(math.Round(255 * (l - a*max(-1, min(k-3, 9-k, 1)))))
	}
	return core.RGB(f(0)<<16 | f(8)<<8 | f(4))
}
