package main

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// The setup page's own controls, drawn for the piano in place of the
// system's look: round and soft, in the start button's coral to orange.

// warm is the start button's coral running to orange, left to right.
var warm = []core.Gradient{core.LinearGradient(90, core.RGB(0xf26b6b), core.RGB(0xf59a3c))}

// The switch: a round knob with a little face on a fat track.
const (
	switchW    core.Points = 60
	switchH    core.Points = 34
	switchKnob core.Points = 28
)

// cuteSwitch shows whether a setting is on: its knob is a little face,
// asleep while off, that springs across and wakes up smiling on a warm
// track when on. The row it sits in takes the press.
func cuteSwitch(cx *core.Scope, on func() bool) core.Node {
	travel := float64(switchW - switchKnob - 6)
	at := func() float64 {
		if on() {
			return travel
		}
		return 0
	}
	x := core.NewAnimatedValue(cx, core.Untracked(cx, at))
	first := true
	cx.Effect(func() {
		to := at()
		if first || core.Untracked(cx, cx.Env().ReduceMotion) {
			first = false
			x.Set(to)
			return
		}
		cx.StartAnimation(core.Spring(x, to, core.SpringConfig{Stiffness: 380, Damping: 18}), nil)
	})
	face := func(svg *core.SVG, shown func() bool, color func() core.Color) core.Node {
		return core.Icon{SVG: svg, StyleFunc: func() core.Style {
			st := core.Style{Position: core.Absolute, Top: core.Pt(4), Left: core.Pt(4), Width: core.Pt(switchKnob - 8), Height: core.Pt(switchKnob - 8), Color: color()}
			if !shown() {
				st.Opacity = core.Alpha(0)
			}
			return st
		}}
	}
	return core.View{
		StyleFunc: func() core.Style {
			st := core.Style{Width: core.Pt(switchW), Height: core.Pt(switchH), CornerRadius: switchH / 2, Padding: core.All(3), FlexShrink: 0,
				Background: tonesOf(cx).track}
			if on() {
				st.Background, st.Gradients = coral, warm
			}
			return st
		},
		Children: []core.Node{core.View{
			Style: core.Style{Width: core.Pt(switchKnob), Height: core.Pt(switchKnob), CornerRadius: switchKnob / 2, Background: core.RGB(0xffffff),
				Shadows:  []core.BoxShadow{{Offset: core.Point{Y: 2}, Blur: 5, Color: core.RGBA(0x3b3355, 0.25)}},
				Animated: &core.AnimatedStyle{TranslateX: x}},
			Children: []core.Node{
				face(faceAsleep, func() bool { return !on() }, func() core.Color { return core.RGB(0xa79fb8) }),
				face(faceAwake, on, func() core.Color { return core.RGB(0xe8554b) }),
			},
		}},
	}
}

// cuteSlider is the volume's slider: a fat track that fills warm behind a
// big round knob with a coral heart, easy for a thumb.
type cuteSlider struct {
	Value    func() float64
	OnChange func(float64)
	Min, Max float64
	Label    string
	TestID   string
}

// Render implements core.Node.
func (s cuteSlider) Render(cx *core.Scope) core.Node {
	const (
		height core.Points = 36
		rail   core.Points = 12
		knob   core.Points = 30
	)
	return base.Slider{
		Value:    func() base.SliderValue { return base.SingleValue(s.Value()) },
		OnChange: func(v base.SliderValue) { s.OnChange(v.End) },
		Min:      func() float64 { return s.Min }, Max: func() float64 { return s.Max },
		Label: s.Label, TestID: s.TestID, ThumbTestID: s.TestID + "-thumb",
		Style:              core.Style{MinWidth: core.Pt(80), Height: core.Pt(height)},
		TrackStyle:         core.Style{Height: core.Pct(100)},
		IndicatorStyle:     core.Style{Height: core.Pt(rail), CornerRadius: rail / 2, Margin: core.Edges{Top: (height - rail) / 2, Left: knob / 2, Right: knob / 2}},
		IndicatorStyleFunc: func() core.Style { return core.Style{Background: tonesOf(cx).track} },
		RangeStyle:         core.Style{CornerRadius: rail / 2, Background: coral, Gradients: warm},
		ThumbStyle: core.Style{Width: core.Pt(knob), Height: core.Pt(knob), CornerRadius: knob / 2,
			Background: core.RGB(0xffffff), BorderWidth: 4, BorderColor: core.RGB(0xffffff),
			Shadows:    []core.BoxShadow{{Offset: core.Point{Y: 3}, Blur: 8, Color: core.RGBA(0x3b3355, 0.28)}},
			Gradients:  []core.Gradient{core.RadialGradient(core.RGB(0xf59a3c), core.RGB(0xf26b6b))},
			Transition: core.Transition{Duration: 120_000_000, Easing: core.EaseOut, Properties: core.TransitionTransform},
			Active:     &core.Style{Transform: core.Transform{Scale: 1.18, TranslateXPercent: -50, TranslateYPercent: -50}}},
	}
}

// tryButton is the little button beside the volume that plays a few notes:
// a soft coral pill with the speaker.
func tryButton(cx *core.Scope, label string, press func()) core.Node {
	return base.Button{
		Label: label, TestID: "try", OnPress: func(core.PressEvent) { press() },
		StyleFunc: func() core.Style {
			tint := float32(0.12)
			if cx.Dark() {
				tint = 0.2
			}
			return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Height: core.Pt(40), Padding: core.Symmetric(0, 16),
				CornerRadius: 20, FlexShrink: 0, Background: accentCoral.of(cx).Faded(tint),
				Active: &core.Style{Transform: core.Transform{Scale: 0.94}, Opacity: core.Alpha(0.85)}}
		},
		Children: []core.Node{
			icon(cx, iconSpeaker, accentCoral, 20),
			core.Text{Content: label, StyleFunc: func() core.Style {
				return ui.TypeBody.Merge(core.Style{FontSize: 15, LineHeight: 20, FontWeight: ui.Bold, Color: accentCoral.of(cx)})
			}},
		},
	}
}
