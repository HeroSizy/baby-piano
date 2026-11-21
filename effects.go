package main

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/nsgo-dev/nsgo/core"

	"github.com/HeroSizy/baby-piano/piano"
)

// Pool sizes: mashing the whole keyboard reuses the oldest burst and glow
// rather than adding views, so it stays smooth.
const (
	burstSlots = 8
	glowSlots  = 4
	sparkCount = 8
	ringCount  = 2
)

// effects draw each note as a firework where it plays: a bright core that
// pops, rings that spread like a shockwave and sparks that fly out and
// fall, with a faint glow of the note's color over the whole screen. Every
// motion runs on the platform from a fixed pool of views.
type effects struct {
	cx     *core.Scope
	size   *core.Signal[core.Size]
	bursts []*burst
	glows  []*glow
	nextB  int
	nextG  int
}

type burst struct {
	at       *core.Signal[core.Point]
	diam     *core.Signal[float64]
	coreTint *core.Signal[core.Color]
	ringTint *core.Signal[core.Color]
	coreO    *core.AnimatedValue
	coreS    *core.AnimatedValue
	rings    [ringCount]struct{ o, s *core.AnimatedValue }
	sparks   [sparkCount]spark
}

type spark struct {
	tint      *core.Signal[core.Color]
	o, tx, ty *core.AnimatedValue
}

type glow struct {
	tint *core.Signal[core.Color]
	o    *core.AnimatedValue
}

func newEffects(cx *core.Scope) *effects {
	e := &effects{cx: cx, size: core.NewSignal(cx, core.Size{})}
	val := func(v float64) *core.AnimatedValue { return core.NewAnimatedValue(cx, v) }
	white := core.RGB(0xffffff)
	for range burstSlots {
		b := &burst{
			at:       core.NewSignal(cx, core.Point{}),
			diam:     core.NewSignal(cx, 100.0),
			coreTint: core.NewSignal(cx, white),
			ringTint: core.NewSignal(cx, white),
			coreO:    val(0),
			coreS:    val(0),
		}
		for i := range b.rings {
			b.rings[i].o, b.rings[i].s = val(0), val(0)
		}
		for i := range b.sparks {
			b.sparks[i] = spark{tint: core.NewSignal(cx, white), o: val(0), tx: val(0), ty: val(0)}
		}
		e.bursts = append(e.bursts, b)
	}
	for range glowSlots {
		e.glows = append(e.glows, &glow{tint: core.NewSignal(cx, white), o: val(0)})
	}
	return e
}

// burst plays note i's firework across the screen by its pitch, low on the
// left.
func (e *effects) burst(i int) {
	s := e.size.GetUntracked()
	t := pitch(i)
	x := float64(s.Width)*(0.08+0.84*t) + rand.Float64()*100 - 50
	y := float64(s.Height) * (0.2 + 0.6*rand.Float64())
	e.burstAt(i, core.Point{X: core.Points(x), Y: core.Points(y)})
}

// pitch is note i's place on the scale, 0 lowest to 1 highest.
func pitch(i int) float64 { return float64(i) / float64(len(piano.Notes)-1) }

// hue is the color of a note at pitch t: red low, through the rainbow, to
// violet high.
func hue(t float64) float64 { return t * 0.85 * 360 }

// burstAt plays note i's firework centred at p. Low notes are bigger and
// slower, high notes smaller and quicker; under reduced motion only the
// core and the glow fade in and out, nothing flies.
func (e *effects) burstAt(i int, p core.Point) {
	t := pitch(i)
	h := hue(t)
	big := 1.45 - 0.75*t // size by pitch
	slow := 1.3 - 0.55*t // duration by pitch
	ms := func(d float64) time.Duration { return time.Duration(d*slow) * time.Millisecond }
	still := core.Untracked(e.cx, e.cx.Env().ReduceMotion)

	e.flash(h, still)

	b := e.bursts[e.nextB]
	e.nextB = (e.nextB + 1) % len(e.bursts)
	d := (90 + rand.Float64()*40) * big
	b.at.Set(p)
	b.diam.Set(d)
	b.coreTint.Set(hsl(h, 1, 0.8))
	b.ringTint.Set(hsl(h, 0.95, 0.62))

	fadeOut := func(v *core.AnimatedValue, after, over float64) *core.Animation {
		return core.Timing(v, 0, core.TimingConfig{Delay: ms(after), Duration: ms(over), Easing: core.BezierEasing(core.EaseOut)})
	}
	if still {
		b.coreS.Set(1)
		b.coreO.Set(0)
		for _, r := range b.rings {
			r.o.Set(0)
		}
		for _, s := range b.sparks {
			s.o.Set(0)
		}
		e.cx.StartAnimation(core.Sequence(
			core.Timing(b.coreO, 0.9, core.TimingConfig{Duration: 200 * time.Millisecond}),
			fadeOut(b.coreO, 0, 900),
		), nil)
		return
	}

	steps := []*core.Animation{
		// The core pops past its size and settles, then fades.
		core.Spring(b.coreS, 1, core.SpringConfig{Stiffness: 260, Damping: 9}),
		core.Sequence(core.Timing(b.coreO, 1, core.TimingConfig{Duration: 60 * time.Millisecond}), fadeOut(b.coreO, 120, 550)),
	}
	b.coreS.Set(0.2)
	for k, r := range b.rings {
		r.s.Set(0.3)
		r.o.Set(0.9 - 0.3*float64(k))
		lag := 90 * float64(k)
		steps = append(steps,
			core.Timing(r.s, 2.6+0.6*float64(k), core.TimingConfig{Delay: ms(lag), Duration: ms(900), Easing: core.BezierEasing(core.EaseOut)}),
			fadeOut(r.o, lag+100, 800),
		)
	}
	start := rand.Float64() * 2 * math.Pi
	for k, s := range b.sparks {
		angle := start + 2*math.Pi*float64(k)/sparkCount + (rand.Float64()-0.5)*0.5
		dist := d * (0.9 + 0.7*rand.Float64())
		s.tint.Set(hsl(h+float64(k%3-1)*28, 1, 0.68))
		s.tx.Set(0)
		s.ty.Set(0)
		s.o.Set(1)
		fly := ms(650 + 250*rand.Float64())
		steps = append(steps,
			core.Timing(s.tx, math.Cos(angle)*dist, core.TimingConfig{Duration: fly, Easing: core.BezierEasing(core.EaseOut)}),
			// Out along the angle, then a little fall.
			core.Sequence(
				core.Timing(s.ty, math.Sin(angle)*dist, core.TimingConfig{Duration: fly, Easing: core.BezierEasing(core.EaseOut)}),
				core.Timing(s.ty, math.Sin(angle)*dist+40*big, core.TimingConfig{Duration: ms(450), Easing: core.BezierEasing(core.EaseIn)}),
			),
			fadeOut(s.o, 300, 800),
		)
	}
	e.cx.StartAnimation(core.Parallel(steps...), nil)
}

// flash tints the whole screen faintly with hue h. Each note takes its own
// layer, so a chord's glows add up instead of replacing one another.
func (e *effects) flash(h float64, still bool) {
	g := e.glows[e.nextG]
	e.nextG = (e.nextG + 1) % len(e.glows)
	g.tint.Set(hsl(h, 0.9, 0.55))
	g.o.Set(0)
	rise := 80 * time.Millisecond
	if still {
		rise = 200 * time.Millisecond
	}
	e.cx.StartAnimation(core.Sequence(
		core.Timing(g.o, 0.07, core.TimingConfig{Duration: rise}),
		core.Timing(g.o, 0, core.TimingConfig{Duration: 700 * time.Millisecond, Easing: core.BezierEasing(core.EaseOut)}),
	), nil)
}

// node is the effects' layer over the whole screen, letting presses
// through.
func (e *effects) node() core.Node {
	fill := core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), PointerEvents: core.PointerEventsNone}
	var nodes []core.Node
	for _, g := range e.glows {
		nodes = append(nodes, core.View{StyleFunc: func() core.Style {
			s := fill
			s.Background = g.tint.Get()
			s.Animated = &core.AnimatedStyle{Opacity: g.o}
			return s
		}})
	}
	// circle is a circle of diameter d centred on its burst's point.
	circle := func(d float64) core.Style {
		return core.Style{Position: core.Absolute, Left: core.Pt(core.Points(-d / 2)), Top: core.Pt(core.Points(-d / 2)),
			Width: core.Pt(core.Points(d)), Height: core.Pt(core.Points(d)), CornerRadius: core.Points(d / 2)}
	}
	for _, b := range e.bursts {
		var parts []core.Node
		for _, r := range b.rings {
			parts = append(parts, core.View{StyleFunc: func() core.Style {
				s := circle(b.diam.Get())
				s.BorderWidth, s.BorderColor = 2, b.ringTint.Get()
				s.Animated = &core.AnimatedStyle{Opacity: r.o, Scale: r.s}
				return s
			}})
		}
		parts = append(parts, core.View{StyleFunc: func() core.Style {
			s := circle(b.diam.Get() * 0.6)
			s.Background = b.coreTint.Get()
			s.ShadowColor, s.ShadowRadius = b.ringTint.Get(), 30
			s.Animated = &core.AnimatedStyle{Opacity: b.coreO, Scale: b.coreS}
			return s
		}})
		for _, sp := range b.sparks {
			parts = append(parts, core.View{StyleFunc: func() core.Style {
				s := circle(10 + b.diam.Get()*0.06)
				s.Background = sp.tint.Get()
				s.Animated = &core.AnimatedStyle{Opacity: sp.o, TranslateX: sp.tx, TranslateY: sp.ty}
				return s
			}})
		}
		nodes = append(nodes, core.View{
			StyleFunc: func() core.Style {
				p := b.at.Get()
				return core.Style{Position: core.Absolute, Left: core.Pt(p.X), Top: core.Pt(p.Y), Width: core.Pt(0), Height: core.Pt(0)}
			},
			Children: parts,
		})
	}
	return core.View{Style: fill, OnLayout: func(b core.Bounds) { e.size.Set(b.Size) }, Children: nodes}
}

// hsl is the color of hue h (degrees), saturation s and lightness l (0 to 1).
func hsl(h, s, l float64) core.Color {
	h = math.Mod(h+360, 360)
	f := func(n float64) uint32 {
		k := math.Mod(n+h/30, 12)
		a := s * min(l, 1-l)
		return uint32(math.Round(255 * (l - a*max(-1, min(k-3, 9-k, 1)))))
	}
	return core.RGB(f(0)<<16 | f(8)<<8 | f(4))
}
