package main

import (
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/core"

	"github.com/HeroSizy/baby-piano/piano"
)

// touchLayout is how a phone or tablet lays out the parents' scale: an
// octave to a row, low at the bottom, from C2 up, so the same column is
// always the same note, as on a piano. Five notes make 5 rows of 5, seven 4
// of 7, all twelve 3 of 12.
func touchLayout(s piano.Scale) (first, cols, rows int) {
	cols = s.PerOctave()
	switch s {
	case piano.Major:
		rows = 4
	case piano.Chromatic:
		rows = 3
	default:
		rows = 5
	}
	return s.From(), cols, rows
}

// touchKeys are big colored keys, an octave to a row. Every finger plays its
// own key, which bounces and glows as it sounds and sends burst its note
// from where the finger landed. All twelve notes look like a piano's octave:
// seven wide white keys, each its own color, with the five black ones
// between them on top.
func (a *app) touchKeys(cx *core.Scope, burst func(note int, at core.Point)) core.Node {
	scale := a.scale()
	first, cols, rows := touchLayout(scale)
	count := cols * rows
	size := core.NewSignal(cx, core.Size{})
	glows := make([]*core.AnimatedValue, count)
	bounces := make([]*core.AnimatedValue, count)
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
		fx, fy := float64(p.X)/float64(s.Width), float64(p.Y)/float64(s.Height)
		row := min(max(int(fy*float64(rows)), 0), rows-1)
		octave := rows - 1 - row
		if scale == piano.Chromatic {
			return octave*12 + pianoKeyAt(fx, fy*float64(rows)-float64(row))
		}
		col := min(max(int(fx*float64(cols)), 0), cols-1)
		return octave*cols + col
	}
	press := func(k int, at core.Point) {
		if k < 0 || !a.play(first+k) {
			return
		}
		glows[k].Set(1)
		cx.StartAnimation(core.Timing(glows[k], 0, core.TimingConfig{Duration: 900 * time.Millisecond, Easing: core.BezierEasing(core.EaseOut)}), nil)
		if !core.Untracked(cx, cx.Env().ReduceMotion) {
			// Squashed, then sprung back past its size and settled.
			bounces[k].Set(0.86)
			cx.StartAnimation(core.Spring(bounces[k], 1, core.SpringConfig{Stiffness: 320, Damping: 8}), nil)
		}
		burst(first+k, at)
	}
	// The keys fill the screen from its top left, so a point in them is
	// where it is on the page.
	a.tapAt = func(p core.Point) { press(keyAt(p), p) }
	cx.Cleanup(func() { a.tapAt = nil })

	// key is key k's look: a rounded color that glows white and bounces
	// when it sounds.
	key := func(k int, color core.Color, glow core.Color, style core.Style) core.Node {
		return core.View{Style: style, Children: []core.Node{core.View{
			Style: core.Style{FlexGrow: 1, CornerRadius: 18, Background: color, ShadowColor: glow, ShadowRadius: 26,
				Animated: &core.AnimatedStyle{Scale: bounces[k], ShadowOpacity: glows[k]}},
			Children: []core.Node{core.View{
				Style: core.Style{FlexGrow: 1, CornerRadius: 18, Background: core.RGB(0xffffff),
					Animated: &core.AnimatedStyle{Opacity: core.Interpolate(glows[k], core.Interpolation{Input: []float64{0, 1}, Output: []float64{0, 0.55}})}},
			}},
		}}}
	}
	// The rows from the top, the highest octave first; a column keeps its
	// color, a little lighter each octave up.
	var keys []core.Node
	for row := rows - 1; row >= 0; row-- {
		light := 0.56 + 0.12*float64(row)/float64(max(rows-1, 1))
		if scale == piano.Chromatic {
			keys = append(keys, pianoOctave(row, light, key))
			continue
		}
		for col := range cols {
			hue := float64(col) / float64(cols) * 330
			keys = append(keys, key(row*cols+col, hsl(hue, 0.78, light), hsl(hue, 1, 0.65),
				core.Style{Width: core.Pct(99.9 / float32(cols)), Height: core.Pct(99.9 / float32(rows)), Padding: core.All(5)}))
		}
	}
	return core.View{
		TestID:   "keys",
		Label:    piano.NoteName(scale.Pitch(first)) + "–" + piano.NoteName(scale.Pitch(first+count-1)),
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

// sharp reports whether MIDI pitch p is a black key's.
func sharp(p int) bool {
	switch p % 12 {
	case 1, 3, 6, 8, 10:
		return true
	}
	return false
}

// whiteSteps and blackSteps are the semitones above C of a piano octave's
// white keys and of its black keys; blackAfter is the white key each black
// one sits after.
var (
	whiteSteps = [7]int{0, 2, 4, 5, 7, 9, 11}
	blackSteps = [5]int{1, 3, 6, 8, 10}
	blackAfter = [5]int{0, 1, 3, 4, 5}
)

// Black keys cover the top blackHeight of an octave's row and blackWidth of
// a white key's width, centered on the line between two white keys.
const (
	blackHeight = 0.58
	blackWidth  = 0.62
)

// pianoKeyAt is the semitone above C under a point at fx across the row and
// fy down it, both from 0 to 1.
func pianoKeyAt(fx, fy float64) int {
	if fy < blackHeight {
		for b, after := range blackAfter {
			if math.Abs(fx-float64(after+1)/7) < blackWidth/2/7 {
				return blackSteps[b]
			}
		}
	}
	return whiteSteps[min(max(int(fx*7), 0), 6)]
}

// pianoOctave is a row of all twelve notes laid out as a piano's octave,
// the key at octave*12+semitone drawn by key.
func pianoOctave(octave int, light float64, key func(k int, color, glow core.Color, style core.Style) core.Node) core.Node {
	var children []core.Node
	for w, step := range whiteSteps {
		hue := float64(w) / 7 * 330
		children = append(children, key(octave*12+step, hsl(hue, 0.78, light), hsl(hue, 1, 0.65),
			core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Padding: core.All(4)}))
	}
	for b, step := range blackSteps {
		hue := (float64(blackAfter[b]) + 0.5) / 7 * 330
		left := (float64(blackAfter[b]+1) - blackWidth/2) / 7 * 100
		children = append(children, key(octave*12+step, hsl(hue, 0.45, 0.22), hsl(hue, 1, 0.65),
			core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pct(float32(left)),
				Width: core.Pct(float32(blackWidth / 7 * 100)), Height: core.Pct(blackHeight * 100), Padding: core.All(4)}))
	}
	return core.View{Style: core.Style{Width: core.Pct(100), Height: core.Pct(99.9 / 3), Direction: core.Row}, Children: children}
}
