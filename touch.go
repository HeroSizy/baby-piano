package main

import (
	"time"

	"github.com/nsgo-dev/nsgo/core"

	"github.com/HeroSizy/baby-piano/piano"
)

// touchFirst and touchCount are the notes a phone or tablet's keys play: the
// 16 from C3 to C6, the piano's middle, comfortable for a baby's ears.
const (
	touchFirst = 10
	touchCount = 16
)

// touchKeys are big colored keys, low at the top left: four rows of four
// held upright, two rows of eight held sideways. Every finger plays its own
// key, which bounces and glows as it sounds and sends burst its note from
// where the finger landed.
func (a *app) touchKeys(cx *core.Scope, burst func(note int, at core.Point)) core.Node {
	size := core.NewSignal(cx, core.Size{})
	cols := func() int {
		if s := size.Get(); s.Width > s.Height {
			return 8
		}
		return 4
	}
	glows := make([]*core.AnimatedValue, touchCount)
	bounces := make([]*core.AnimatedValue, touchCount)
	for i := range glows {
		glows[i] = core.NewAnimatedValue(cx, 0)
		bounces[i] = core.NewAnimatedValue(cx, 1)
	}
	// keyAt is the key under p, or -1 in the parents' corner.
	keyAt := func(p core.Point) int {
		s := size.GetUntracked()
		if s.Width == 0 || s.Height == 0 || (float64(p.X) > float64(s.Width)-cornerSize && p.Y < cornerSize) {
			return -1
		}
		c := cols()
		rows := touchCount / c
		col := min(int(float64(p.X)/float64(s.Width)*float64(c)), c-1)
		row := min(int(float64(p.Y)/float64(s.Height)*float64(rows)), rows-1)
		return row*c + col
	}
	press := func(k int, at core.Point) {
		if k < 0 || !a.play(touchFirst+k) {
			return
		}
		glows[k].Set(1)
		cx.StartAnimation(core.Timing(glows[k], 0, core.TimingConfig{Duration: 900 * time.Millisecond, Easing: core.BezierEasing(core.EaseOut)}), nil)
		if !core.Untracked(cx, cx.Env().ReduceMotion) {
			// Squashed, then sprung back past its size and settled.
			bounces[k].Set(0.86)
			cx.StartAnimation(core.Spring(bounces[k], 1, core.SpringConfig{Stiffness: 320, Damping: 8}), nil)
		}
		burst(touchFirst+k, at)
	}
	// The keys fill the screen from its top left, so a point in them is
	// where it is on the page.
	a.tapAt = func(p core.Point) { press(keyAt(p), p) }
	cx.Cleanup(func() { a.tapAt = nil })

	var keys []core.Node
	for k := range touchCount {
		t := float64(k) / float64(touchCount-1)
		hue := t * 0.85 * 360
		keys = append(keys, core.View{
			StyleFunc: func() core.Style {
				return core.Style{Width: core.Pct(100 / float32(cols())), Height: core.Pct(100 / float32(touchCount/cols())), Padding: core.All(6)}
			},
			Children: []core.Node{core.View{
				Style: core.Style{FlexGrow: 1, CornerRadius: 22, Background: hsl(hue, 0.75, 0.62),
					ShadowColor: hsl(hue, 1, 0.65), ShadowRadius: 26,
					Animated: &core.AnimatedStyle{Scale: bounces[k], ShadowOpacity: glows[k]}},
				Children: []core.Node{core.View{
					Style: core.Style{FlexGrow: 1, CornerRadius: 22, Background: core.RGB(0xffffff),
						Animated: &core.AnimatedStyle{Opacity: core.Interpolate(glows[k], core.Interpolation{Input: []float64{0, 1}, Output: []float64{0, 0.55}})}},
				}},
			}},
		})
	}
	return core.View{
		TestID:   "keys",
		Label:    piano.Notes[touchFirst] + "–" + piano.Notes[touchFirst+touchCount-1],
		OnLayout: func(b core.Bounds) { size.Set(b.Size) },
		OnTouch: func(e core.TouchEvent) {
			if e.Phase != core.TouchStart {
				return
			}
			for _, t := range e.Touches {
				if t.Changed {
					press(keyAt(t.Position), t.PagePosition)
				}
			}
		},
		// To the screen's edges, clear of a notch or camera cutout; the
		// bottom's inset is only the home indicator, which hides.
		Style: core.Style{FlexGrow: 1, Direction: core.Row, Wrap: core.Wrap, Padding: core.All(6),
			PaddingEnv: core.EnvEdges{Top: core.EnvSafeArea, Right: core.EnvSafeArea, Left: core.EnvSafeArea}},
		Children: keys,
	}
}
