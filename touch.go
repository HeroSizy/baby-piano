package main

import (
	"math"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

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

	// letter is how big a key's note name is drawn: its bubble's height,
	// about half the smaller side of a white key, kept readable on a phone
	// and not too big on a tablet.
	letters := a.settings.noteNames.Get()
	across := cols
	if scale == piano.Chromatic {
		across = 7
	}
	// All twelve keep it below the black keys.
	letter := func() float64 {
		s := size.Get()
		w, h := float64(s.Width)/float64(across), float64(s.Height)/float64(rows)
		if scale == piano.Chromatic {
			h *= (1 - blackHeight) * 1.6
		}
		return min(max(min(w, h)*0.42, 26), 64)
	}
	lift := 0.36
	if scale == piano.Chromatic {
		lift = 0.18
	}

	// key is key k's look: a rounded color that glows white and bounces
	// when it sounds, its color as the screen is light or dark, and with
	// the parents' say its note's letter on it.
	key := func(k int, look keyLook, style core.Style) core.Node {
		radii := look.corners()
		children := []core.Node{core.View{
			Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0),
				CornerRadii: radii, Background: core.RGB(0xffffff),
				Animated: &core.AnimatedStyle{Opacity: core.Interpolate(glows[k], core.Interpolation{Input: []float64{0, 1}, Output: []float64{0, 0.55}})}},
		}}
		if letters {
			name := piano.Letter(scale.Pitch(first + k))
			children = append(children, noteLetter(cx, name, look, func() float64 {
				if !look.black {
					return letter() * 0.85
				}
				// As big as fits the black key's width.
				inner := float64(size.Get().Width)/7*blackWidth - 12
				return min(letter()*0.62, inner/1.25)
			}))
		}
		face := core.Node(core.View{
			StyleFunc: func() core.Style {
				st := core.Style{FlexGrow: 1, CornerRadii: radii, Background: look.color(cx.Dark()), Gradients: look.sheen(cx.Dark()),
					ShadowColor: look.glow(), ShadowRadius: 26,
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyEnd,
					Padding:  core.Edges{Bottom: core.Points(letter() * lift)},
					Animated: &core.AnimatedStyle{Scale: bounces[k], ShadowOpacity: glows[k]}}
				if look.black && cx.Dark() {
					// On the night a black key is a lighter plum with a pale
					// rim, so it stands off the background between the keys.
					st.BorderWidth, st.BorderColor = 1.5, core.RGBA(0xffffff, 0.22)
				}
				return st
			},
			Children: children,
		})
		if look.black {
			// A black key stands above the colored ones: it casts a
			// shadow on them.
			face = core.View{Style: core.Style{FlexGrow: 1, CornerRadii: radii,
				Shadows: []core.BoxShadow{{Offset: core.Point{Y: 3}, Blur: 8, Color: core.RGBA(0x1a1030, 0.35)}}}, Children: []core.Node{face}}
		}
		return core.View{Style: style, Children: []core.Node{face}}
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
			keys = append(keys, key(row*cols+col, keyLook{hue: hue, light: light},
				core.Style{Width: core.Pct(99.9 / float32(cols)), Height: core.Pct(99.9 / float32(rows)), Padding: core.All(5)}))
		}
	}
	return core.View{
		TestID:   "keys",
		Label:    piano.NoteName(scale.Pitch(first)) + "–" + piano.NoteName(scale.Pitch(first+count-1)),
		OnLayout: func(b core.Bounds) { animateTurn(cx, size.GetUntracked(), b.Size); size.Set(b.Size) },
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

// keyLook is a key's color: its hue, and how light its row is, or a piano's
// black key.
type keyLook struct {
	hue, light float64
	black      bool
}

// color is the key's color on the dark screen, as bright as the night
// lets it glow, or on the light one, a pastel a step lighter that sits on
// the paper; a black key is plum ink on either, lighter than the night on
// the dark screen so it shows against it, whatever its hue, which is only
// its glow's.
func (l keyLook) color(dark bool) core.Color {
	switch {
	case l.black && dark:
		return core.RGB(0x4a406c)
	case l.black:
		return core.RGB(0x3b3254)
	case dark:
		return hsl(l.hue, 0.78, l.light)
	}
	return hsl(l.hue, 0.82, l.light+0.08)
}

// glow is the color the key glows when it sounds.
func (l keyLook) glow() core.Color { return hsl(l.hue, 1, 0.65) }

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
func pianoOctave(octave int, light float64, key func(k int, look keyLook, style core.Style) core.Node) core.Node {
	var children []core.Node
	for w, step := range whiteSteps {
		hue := float64(w) / 7 * 330
		children = append(children, key(octave*12+step, keyLook{hue: hue, light: light},
			core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Padding: core.All(4)}))
	}
	for b, step := range blackSteps {
		hue := (float64(blackAfter[b]) + 0.5) / 7 * 330
		left := (float64(blackAfter[b]+1) - blackWidth/2) / 7 * 100
		children = append(children, key(octave*12+step, keyLook{hue: hue, black: true},
			core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pct(float32(left)),
				Width: core.Pct(float32(blackWidth / 7 * 100)), Height: core.Pct(blackHeight * 100), Padding: core.All(4)}))
	}
	return core.View{Style: core.Style{Width: core.Pct(100), Height: core.Pct(99.9 / 3), Direction: core.Row}, Children: children}
}

// noteLetter is a key's note name, as C or F#, pressed into the key as if
// printed there. On a colored key it is a deeper shade of the key's own
// color with a soft light edge below; on a black key a pale lilac with a
// shade above, its sharp small and raised as music writes it. size is its
// font size.
func noteLetter(cx *core.Scope, name string, look keyLook, size func() float64) core.Node {
	text := func(s string, scale float64) core.Node {
		return core.Text{Content: s, StyleFunc: func() core.Style {
			pt := size() * scale
			edge := core.Point{Y: core.Points(max(pt*0.025, 0.8))}
			rim := core.RGBA(0xffffff, 0.42)
			switch {
			case look.black:
				edge.Y, rim = -edge.Y, core.RGBA(0x000000, 0.5)
			case cx.Dark():
				rim = core.RGBA(0xffffff, 0.28)
			}
			return ui.TypeBody.Merge(core.Style{FontSize: core.Points(pt), LineHeight: core.Points(pt * 1.05), FontWeight: ui.Bold,
				Color: look.ink(cx.Dark()), TextShadowColor: rim, TextShadowOffset: edge, TextShadowRadius: core.Points(pt * 0.03)})
		}}
	}
	letter, sharp := strings.CutSuffix(name, "#")
	if !sharp {
		return text(name, 1)
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignStart}, Children: []core.Node{
		text(letter, 1), text("#", 0.62),
	}}
}

// ink is the color a key's letter is pressed in: the key's own, deeper, or
// on a black key a pale lilac.
func (l keyLook) ink(dark bool) core.Color {
	switch {
	case l.black && dark:
		return core.RGBA(0xe6dff7, 0.78)
	case l.black:
		return core.RGBA(0xe6dff7, 0.72)
	case dark:
		return hsl(l.hue, 0.72, l.light-0.17).Faded(0.8)
	}
	return hsl(l.hue, 0.66, l.light-0.12).Faded(0.8)
}

// corners are a key's: round all over, or a black key's, squarer at the
// top where it meets the screen's edge of the octave and round at its foot.
func (l keyLook) corners() core.Corners {
	if l.black {
		return core.Corners{TopLeft: 8, TopRight: 8, BottomRight: 16, BottomLeft: 16}
	}
	return core.Corners{TopLeft: 18, TopRight: 18, BottomRight: 18, BottomLeft: 18}
}

// sheen is the light down a black key, a little brighter at its top; the
// colored keys are flat.
func (l keyLook) sheen(dark bool) []core.Gradient {
	switch {
	case !l.black:
		return nil
	case dark:
		return []core.Gradient{core.LinearGradient(180, core.RGB(0x62568a), core.RGB(0x3f355e))}
	}
	return []core.Gradient{core.LinearGradient(180, core.RGB(0x4d4368), core.RGB(0x2f2745))}
}
