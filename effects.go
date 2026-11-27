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
	sparkCount = 14
	ringCount  = 1
)

// effects draw each note as a firework where it plays: a small flash, a
// faint ring and a spray of sparks, with a faint glow of the note's color
// over the whole screen. No two are alike: each note picks one of a few
// kinds of firework (fireKinds), and its sparks their own shapes, sizes,
// shades, reach and timing. With
// the fireworks off, a note is a soft bloom of its color instead, so a
// press still shows. Every motion runs on the platform from a fixed pool
// of views, in colors for the light or dark screen.
type effects struct {
	cx     *core.Scope
	fancy  bool // fireworks, rather than the soft bloom
	size   *core.Signal[core.Size]
	bursts []*burst
	glows  []*glow
	nextB  int
	nextG  int
}

type burst struct {
	at       *core.Signal[core.Point]
	diam     *core.Signal[float64]
	coreDiam *core.Signal[float64]
	coreTint *core.Signal[core.Color]
	ringTint *core.Signal[core.Color]
	coreO    *core.AnimatedValue
	coreS    *core.AnimatedValue
	rings    [ringCount]struct{ o, s *core.AnimatedValue }
	sparks   [sparkCount]spark
}

// spark is one piece of a firework: a shape of a size and a shade that
// flies, turns, shrinks and fades.
type spark struct {
	shape              *core.Signal[*core.SVG]
	size               *core.Signal[float64]
	tint               *core.Signal[core.Color]
	o, tx, ty, rot, sc *core.AnimatedValue
}

type glow struct {
	tint *core.Signal[core.Color]
	o    *core.AnimatedValue
}

func newEffects(cx *core.Scope, fancy bool) *effects {
	e := &effects{cx: cx, fancy: fancy, size: core.NewSignal(cx, core.Size{})}
	val := func(v float64) *core.AnimatedValue { return core.NewAnimatedValue(cx, v) }
	white := core.RGB(0xffffff)
	for range burstSlots {
		b := &burst{
			at:       core.NewSignal(cx, core.Point{}),
			diam:     core.NewSignal(cx, 100.0),
			coreDiam: core.NewSignal(cx, 60.0),
			coreTint: core.NewSignal(cx, white),
			ringTint: core.NewSignal(cx, white),
			coreO:    val(0),
			coreS:    val(0),
		}
		for i := range b.rings {
			b.rings[i].o, b.rings[i].s = val(0), val(0)
		}
		for i := range b.sparks {
			b.sparks[i] = spark{shape: core.NewSignal(cx, sparkDot), size: core.NewSignal(cx, 12.0), tint: core.NewSignal(cx, white),
				o: val(0), tx: val(0), ty: val(0), rot: val(0), sc: val(1)}
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

// fireTones are a firework's colors at hue h: on the dark screen bright
// and pale, glowing; on the light one deeper, so they stand out from the
// paper.
type fireTones struct {
	core, ring, glow   core.Color
	pale               core.Color // the palest spark: near white on the night, the note's deepest on paper
	bloom              core.Color // the soft bloom's, with the fireworks off
	sparkS, sparkL     float64
	glowPeak, softPeak float64 // the screen's tint at its strongest, with and without fireworks
}

func fireTonesAt(h float64, dark bool) fireTones {
	if dark {
		return fireTones{core: hsl(h, 1, 0.8), ring: hsl(h, 0.95, 0.62), glow: hsl(h, 0.9, 0.55), bloom: hsl(h, 0.9, 0.68), pale: hsl(h, 1, 0.94),
			sparkS: 1, sparkL: 0.78, glowPeak: 0.07, softPeak: 0.09}
	}
	return fireTones{core: hsl(h, 1, 0.62), ring: hsl(h, 0.9, 0.5), glow: hsl(h, 0.9, 0.6), bloom: hsl(h, 0.95, 0.6), pale: hsl(h, 0.9, 0.36),
		sparkS: 1, sparkL: 0.5, glowPeak: 0.10, softPeak: 0.10}
}

// burstAt plays note i at p: its firework, or with the fireworks off its
// soft bloom. Low notes are bigger and slower, high notes smaller and
// quicker; under reduced motion only the core and the glow fade in and
// out, nothing flies.
func (e *effects) burstAt(i int, p core.Point) {
	t := pitch(i)
	h := hue(t)
	big := 1.45 - 0.75*t // size by pitch
	slow := 1.3 - 0.55*t // duration by pitch
	ms := func(d float64) time.Duration { return time.Duration(d*slow) * time.Millisecond }
	still := core.Untracked(e.cx, e.cx.Env().ReduceMotion)
	tn := fireTonesAt(h, core.Untracked(e.cx, e.cx.Dark))

	peak := tn.glowPeak
	if !e.fancy {
		peak = tn.softPeak
	}
	e.flash(tn.glow, peak, still)

	b := e.bursts[e.nextB]
	e.nextB = (e.nextB + 1) % len(e.bursts)
	d := (90 + rand.Float64()*40) * big
	b.at.Set(p)
	b.diam.Set(d)
	b.coreDiam.Set(d * 0.6)
	b.coreTint.Set(tn.core)
	b.ringTint.Set(tn.ring)

	fadeOut := func(v *core.AnimatedValue, after, over float64) *core.Animation {
		return core.Timing(v, 0, core.TimingConfig{Delay: ms(after), Duration: ms(over), Easing: core.BezierEasing(core.EaseOut)})
	}
	if !e.fancy {
		// The soft bloom: the core alone, larger and fainter, swelling
		// slowly as it fades; still, it only fades in and out.
		b.coreDiam.Set(d * 1.7 * 0.6)
		b.coreTint.Set(tn.bloom)
		for _, r := range b.rings {
			r.o.Set(0)
		}
		for _, s := range b.sparks {
			s.o.Set(0)
		}
		b.coreO.Set(0)
		if still {
			b.coreS.Set(1)
		} else {
			b.coreS.Set(0.55)
		}
		steps := []*core.Animation{core.Sequence(
			core.Timing(b.coreO, 0.6, core.TimingConfig{Duration: 120 * time.Millisecond}),
			fadeOut(b.coreO, 80, 900),
		)}
		if !still {
			steps = append(steps, core.Timing(b.coreS, 1.15, core.TimingConfig{Duration: ms(1000), Easing: core.BezierEasing(core.EaseOut)}))
		}
		e.cx.StartAnimation(core.Parallel(steps...), nil)
		return
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

	// The flash: small, quick, gone before the sparks have landed.
	b.coreDiam.Set(d * 0.34)
	b.coreS.Set(0.3)
	steps := []*core.Animation{
		core.Timing(b.coreS, 1.5, core.TimingConfig{Duration: ms(420), Easing: burstEase}),
		core.Sequence(core.Timing(b.coreO, 0.95, core.TimingConfig{Duration: 50 * time.Millisecond}), fadeOut(b.coreO, 40, 330)),
	}
	// One faint ring, so the burst has a breath of air round it.
	for _, r := range b.rings {
		r.s.Set(0.25)
		r.o.Set(0.3)
		steps = append(steps,
			core.Timing(r.s, 1.9+0.8*rand.Float64(), core.TimingConfig{Duration: ms(800), Easing: burstEase}),
			fadeOut(r.o, 60, 620),
		)
	}

	kind := fireKinds[rand.IntN(len(fireKinds))]
	n := kind.count[0] + rand.IntN(kind.count[1]-kind.count[0]+1)
	start := rand.Float64() * 2 * math.Pi
	for k := range b.sparks {
		s := &b.sparks[k]
		if k >= n {
			s.o.Set(0)
			continue
		}
		// Round the burst, but never evenly: each spark strays within
		// its share of the circle, and by more than its share in a cloud.
		angle := start + 2*math.Pi*(float64(k)+rand.Float64()*kind.stray)/float64(n)
		reach := kind.reach[0] + (kind.reach[1]-kind.reach[0])*rand.Float64()
		if kind.layers && k%2 == 1 {
			reach *= 0.55
		}
		dist := d * reach
		x, y := math.Cos(angle)*dist, math.Sin(angle)*dist*kind.squash-d*kind.lift
		shape := kind.shapes[rand.IntN(len(kind.shapes))]
		size := (kind.size[0] + (kind.size[1]-kind.size[0])*rand.Float64()) * (0.75 + 0.35*big)
		s.shape.Set(shape)
		s.size.Set(size)
		s.tint.Set(sparkTint(h, tn))

		wait := time.Duration(rand.Float64()*kind.spread) * time.Millisecond
		fly := ms(kind.fly * (0.8 + 0.4*rand.Float64()))
		fall := ms(kind.fall * (0.8 + 0.4*rand.Float64()))
		drop := kind.drop * big * (0.6 + 0.8*rand.Float64())

		// A streak points the way it flies; the others tumble.
		if shape == sparkStreak {
			s.rot.Set(angle * 180 / math.Pi)
		} else {
			from := rand.Float64() * 360
			s.rot.Set(from)
			steps = append(steps, core.Timing(s.rot, from+(rand.Float64()-0.5)*kind.spin, core.TimingConfig{Delay: wait, Duration: fly + fall}))
		}
		s.tx.Set(x * kind.from)
		s.ty.Set(y * kind.from)
		s.sc.Set(0.2)
		s.o.Set(0)
		shine := []*core.Animation{core.Timing(s.o, 1, core.TimingConfig{Delay: wait, Duration: 40 * time.Millisecond})}
		if kind.twinkle {
			// It glitters on the way: dim and bright again, at its own pace.
			for range 2 + rand.IntN(2) {
				blink := time.Duration(70+rand.IntN(90)) * time.Millisecond
				shine = append(shine,
					core.Timing(s.o, 0.25, core.TimingConfig{Duration: blink}),
					core.Timing(s.o, 1, core.TimingConfig{Duration: blink}))
			}
		}
		shine = append(shine, core.Timing(s.o, 0, core.TimingConfig{Delay: fly * 4 / 10, Duration: fly*3/10 + fall, Easing: core.BezierEasing(core.EaseIn)}))
		steps = append(steps,
			core.Sequence(shine...),
			// Out fast and slowing, then drifting on as it falls.
			core.Sequence(
				core.Timing(s.tx, x*0.94, core.TimingConfig{Delay: wait, Duration: fly, Easing: burstEase}),
				core.Timing(s.tx, x, core.TimingConfig{Duration: fall}),
			),
			core.Sequence(
				core.Timing(s.ty, y, core.TimingConfig{Delay: wait, Duration: fly, Easing: burstEase}),
				core.Timing(s.ty, y+drop, core.TimingConfig{Duration: fall, Easing: core.BezierEasing(core.EaseIn)}),
			),
			// It pops to its size, then dwindles as it falls.
			core.Sequence(
				core.Spring(s.sc, 1, core.SpringConfig{Stiffness: 300, Damping: 12}),
				core.Timing(s.sc, 0.35, core.TimingConfig{Delay: wait + fly/2, Duration: fall, Easing: core.BezierEasing(core.EaseIn)}),
			),
		)
	}
	e.cx.StartAnimation(core.Parallel(steps...), nil)
}

// burstEase is a firework's rush out: nearly all the way at once, then
// coasting to a stop.
var burstEase = core.BezierEasing(core.Curve{X1: 0.08, Y1: 0.75, X2: 0.2, Y2: 1})

// The sparks' shapes, filled in the spark's color.
var (
	sparkDot = fillSVG(`<circle cx="12" cy="12" r="7"/>`)
	// sparkStar is a plump five-pointed star.
	sparkStar = fillSVG(`<path d="M12 2.6l2.7 5.7 6.2.8-4.5 4.4 1.1 6.2L12 16.8l-5.5 2.9 1.1-6.2L3.1 9.1l6.2-.8z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>`)
	// sparkGlint is a four-pointed glint, as a star twinkles.
	sparkGlint = fillSVG(`<path d="M12 2C12.8 8 16 11.2 22 12 16 12.8 12.8 16 12 22 11.2 16 8 12.8 2 12 8 11.2 11.2 8 12 2z"/>`)
	sparkHeart = fillSVG(`<path d="M12 20.5C6.500 16.200 3 13 3 9.200 3 6.300 5.200 4.200 7.900 4.200c1.600 0 3.100.8 4.100 2.100 1-1.300 2.500-2.100 4.100-2.100 2.700 0 4.900 2.100 4.900 5 0 3.800-3.500 7-9 11.300z"/>`)
	// sparkStreak is a short trail, long the way it flies.
	sparkStreak = fillSVG(`<rect x="1" y="9.500" width="22" height="5" rx="2.500"/>`)
)

func fillSVG(body string) *core.SVG {
	s, err := core.ParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="currentColor">` + body + `</svg>`))
	if err != nil {
		panic(err)
	}
	return s
}

// fireKind is a kind of firework: how many sparks, of which shapes and
// sizes, how far and how they fly. Lengths are in the burst's diameters,
// times in milliseconds for a middle note.
type fireKind struct {
	shapes  []*core.SVG
	count   [2]int     // fewest and most sparks
	size    [2]float64 // smallest and largest, in points
	reach   [2]float64 // nearest and farthest
	layers  bool       // every other spark stops short, an inner round
	stray   float64    // how far a spark strays from its place round the circle: 1 its own share
	from    float64    // where it starts along its way: 0 the middle, more for a cloud
	squash  float64    // 1 a round burst, less a flatter one
	lift    float64    // how far the whole burst rises
	spread  float64    // the sparks leave over this long
	fly     float64
	fall    float64
	drop    float64 // how far it falls, in points
	spin    float64 // how far it turns, in degrees
	twinkle bool
}

var fireKinds = []fireKind{
	// A round burst of dots and little stars in two rounds.
	{shapes: []*core.SVG{sparkDot, sparkDot, sparkStar}, count: [2]int{11, 14}, size: [2]float64{12, 22}, reach: [2]float64{0.68, 1.1},
		layers: true, stray: 0.8, squash: 1, spread: 50, fly: 520, fall: 520, drop: 34, spin: 200},
	// Long trails that hang and droop, like a willow.
	{shapes: []*core.SVG{sparkStreak}, count: [2]int{10, 14}, size: [2]float64{21, 36}, reach: [2]float64{0.72, 1.2},
		stray: 0.9, squash: 1, spread: 30, fly: 600, fall: 700, drop: 70},
	// A cloud of glints that twinkle where they hang.
	{shapes: []*core.SVG{sparkGlint, sparkGlint, sparkDot}, count: [2]int{9, 13}, size: [2]float64{13, 31}, reach: [2]float64{0.22, 1.1},
		stray: 6, from: 0.55, squash: 0.85, spread: 260, fly: 620, fall: 420, drop: 14, spin: 60, twinkle: true},
	// A few big stars that tumble out.
	{shapes: []*core.SVG{sparkStar}, count: [2]int{6, 9}, size: [2]float64{23, 39}, reach: [2]float64{0.65, 1},
		stray: 0.6, squash: 1, spread: 40, fly: 560, fall: 560, drop: 40, spin: 420},
	// Hearts and dots thrown up like a fountain, falling back.
	{shapes: []*core.SVG{sparkHeart, sparkHeart, sparkDot}, count: [2]int{7, 10}, size: [2]float64{16, 34}, reach: [2]float64{0.36, 0.94},
		stray: 1.4, squash: 0.6, lift: 0.5, spread: 120, fly: 560, fall: 680, drop: 90, spin: 90},
	// Everything at once, a little of each, glittering.
	{shapes: []*core.SVG{sparkDot, sparkStar, sparkGlint, sparkStreak}, count: [2]int{12, 14}, size: [2]float64{12, 29}, reach: [2]float64{0.43, 1.2},
		stray: 1.2, squash: 1, spread: 90, fly: 560, fall: 560, drop: 44, spin: 240, twinkle: true},
}

// sparkTint is a spark's shade for a note of hue h: mostly near the note's
// color, lighter or deeper, some a neighbor's, and now and then a warm
// gold or the screen's palest, as fireworks glitter.
func sparkTint(h float64, tn fireTones) core.Color {
	switch r := rand.Float64(); {
	case r < 0.12:
		return hsl(44, 1, tn.sparkL+0.04)
	case r < 0.26:
		return tn.pale
	case r < 0.45:
		return hsl(h+(rand.Float64()-0.5)*110, tn.sparkS, tn.sparkL+(rand.Float64()-0.5)*0.1)
	}
	return hsl(h+(rand.Float64()-0.5)*36, tn.sparkS, tn.sparkL+(rand.Float64()-0.5)*0.16)
}

// flash tints the whole screen faintly with color c, up to opacity peak.
// Each note takes its own layer, so a chord's glows add up instead of
// replacing one another.
func (e *effects) flash(c core.Color, peak float64, still bool) {
	g := e.glows[e.nextG]
	e.nextG = (e.nextG + 1) % len(e.glows)
	g.tint.Set(c)
	g.o.Set(0)
	rise := 80 * time.Millisecond
	if still {
		rise = 200 * time.Millisecond
	}
	e.cx.StartAnimation(core.Sequence(
		core.Timing(g.o, peak, core.TimingConfig{Duration: rise}),
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
			s := circle(b.coreDiam.Get())
			s.Background = b.coreTint.Get()
			s.ShadowColor, s.ShadowRadius = b.ringTint.Get(), 24
			s.Animated = &core.AnimatedStyle{Opacity: b.coreO, Scale: b.coreS}
			return s
		}})
		for i := range b.sparks {
			sp := &b.sparks[i]
			parts = append(parts, core.Icon{SVGFunc: sp.shape.Get, StyleFunc: func() core.Style {
				d := core.Points(sp.size.Get())
				return core.Style{Position: core.Absolute, Left: core.Pt(-d / 2), Top: core.Pt(-d / 2), Width: core.Pt(d), Height: core.Pt(d),
					Color:    sp.tint.Get(),
					Animated: &core.AnimatedStyle{Opacity: sp.o, TranslateX: sp.tx, TranslateY: sp.ty, Rotate: sp.rot, Scale: sp.sc}}
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
