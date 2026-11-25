package main

import (
	"slices"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/HeroSizy/baby-piano/kiosk"
	"github.com/HeroSizy/baby-piano/locales"
)

// outputSection is where parents choose what the piano plays through, in
// the settings card below the volume: on an iPhone or iPad, where it plays
// now and the system's AirPlay button; on a computer, its outputs to pick
// from. Android, which moves the sound to headphones itself and picks other
// outputs in its own media panel, and computers that list none show
// nothing.
func (a *app) outputSection(cx *core.Scope, heading func(string) core.Node) []core.Node {
	if cx.Env().OS() == core.OSIOS {
		return []core.Node{divider(cx), heading(locales.Output(cx)), routeRow(cx)}
	}
	if !a.kiosk.Supported() {
		return nil
	}
	outputs := core.NewSignalFunc(cx, a.kiosk.Outputs(), slices.Equal[[]kiosk.Output])
	// Outputs come and go, headphones plugged in, a Bluetooth speaker
	// turned on, and the system may switch to them: look every second.
	var poll func()
	poll = func() {
		outputs.Set(a.kiosk.Outputs())
		cx.AfterFunc(time.Second, poll)
	}
	cx.AfterFunc(time.Second, poll)
	choose := func(id string) {
		a.kiosk.SetOutput(id)
		outputs.Set(a.kiosk.Outputs())
	}
	return []core.Node{shownWhen(func() bool { return len(outputs.Get()) > 0 }, core.View{Style: core.Style{Gap: 16}, Children: []core.Node{
		divider(cx),
		heading(locales.Output(cx)),
		core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
			core.Each(outputs.Get, func(o kiosk.Output) string { return o.ID }, func(cx *core.Scope, o func() kiosk.Output, _ func() int) core.Node {
				return outputRow(cx, o, choose)
			}),
		}},
	}})}
}

func shownWhen(when func() bool, node core.Node) core.Node {
	return core.Show{When: when, Then: func(*core.Scope) core.Node { return node }}
}

// outputRow is one of a computer's outputs, the one playing lifted in the
// accent's tint and edge, as the chosen scale is.
func outputRow(cx *core.Scope, o func() kiosk.Output, choose func(id string)) core.Node {
	chosen := func() bool { return o().Default }
	return base.Button{
		Role: core.RoleRadio, Selected: chosen, LabelFunc: func() string { return o().Name }, TestID: "output",
		OnPress: func(core.PressEvent) { choose(o().ID) },
		StyleFunc: func() core.Style {
			t := tonesOf(cx)
			st := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(10, 14), CornerRadius: 14,
				BorderWidth: 2, BorderColor: t.tileEdge, Background: t.tile, Active: &core.Style{Opacity: core.Alpha(0.8)}}
			if chosen() {
				st.BorderColor, st.Background = coral, t.chosen
			}
			return st
		},
		Children: []core.Node{
			core.Icon{SVGFunc: func() *core.SVG { return outputIcon(o().Name) }, StyleFunc: func() core.Style {
				c := tonesOf(cx).muted
				if chosen() {
					c = accentCoral.of(cx)
				}
				return core.Style{Width: core.Pt(22), Height: core.Pt(22), FlexShrink: 0, Color: c}
			}},
			core.Text{ContentFunc: func() string { return o().Name }, StyleFunc: func() core.Style {
				w := ui.Regular
				if chosen() {
					w = ui.Semibold
				}
				return inked(cx, core.Style{FontSize: 15, LineHeight: 20, FontWeight: w, FlexGrow: 1, FlexShrink: 1})()
			}},
			// A radio's dot: a ring, filled while it plays here.
			core.View{StyleFunc: func() core.Style {
				t := tonesOf(cx)
				st := core.Style{Width: core.Pt(20), Height: core.Pt(20), CornerRadius: 10, FlexShrink: 0, BorderWidth: 2, BorderColor: t.track,
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
				if chosen() {
					st.BorderColor = coral
				}
				return st
			}, Children: []core.Node{shownWhen(chosen, core.View{Style: core.Style{Width: core.Pt(10), Height: core.Pt(10), CornerRadius: 5, Background: coral}})}},
		},
	}
}

// routeRow says where an iPhone or iPad plays now, beside the system's
// AirPlay button, which lists the outputs and moves the sound to the one
// chosen: its speaker, headphones, a Bluetooth speaker, AirPlay.
func routeRow(cx *core.Scope) core.Node {
	name := core.NewSignal(cx, outputName())
	var poll func()
	poll = func() {
		if n := outputName(); n != name.GetUntracked() {
			name.Set(n)
		}
		cx.AfterFunc(time.Second, poll)
	}
	cx.AfterFunc(time.Second, poll)
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 14}, Children: []core.Node{
		core.View{StyleFunc: func() core.Style { return core.Style{FlexShrink: 0} }, Children: []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node { return badge(cx, outputIcon(name.Get()), accentBlue, 44, 24, 14) }),
		}},
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
			core.Text{ContentFunc: func() string {
				if n := name.Get(); n != "" {
					return locales.OutputPlayingOn(cx, n)
				}
				return locales.OutputChoose(cx)
			}, StyleFunc: inked(cx, core.Style{FontSize: 15, LineHeight: 20, FontWeight: ui.Semibold}), TestID: "output"},
			core.Text{Content: locales.OutputNote(cx), StyleFunc: hushed(cx, core.Style{FontSize: 13, LineHeight: 18})},
		}},
		core.Native{
			Component: routePickerComponent,
			PropsFunc: func() map[string]any { return map[string]any{"dark": cx.Dark()} },
			Label:     locales.OutputChoose(cx),
			TestID:    "airplay",
			StyleFunc: func() core.Style {
				return core.Style{Width: core.Pt(44), Height: core.Pt(44), FlexShrink: 0, CornerRadius: 22, Background: tonesOf(cx).tile}
			},
		},
	}}
}

// outputIcon is headphones for an output whose name says so, AirPods and
// Beats among them, and a speaker for any other.
func outputIcon(name string) *core.SVG {
	for _, w := range []string{"headphone", "Headphone", "AirPods", "Beats", "earbud", "Earbud", "Kopfh", "耳机", "耳機", "イヤホン", "ヘッドホン", "헤드폰", "이어폰", "casque", "Casque", "auricular", "Auricular", "fone"} {
		if strings.Contains(name, w) {
			return iconHeadphones
		}
	}
	return iconSpeaker
}
