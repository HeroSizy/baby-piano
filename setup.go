package main

import (
	"errors"
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/HeroSizy/baby-piano/kiosk"
	"github.com/HeroSizy/baby-piano/locales"
	"github.com/HeroSizy/baby-piano/piano"
)

var errSecureInput = errors.New("secure keyboard entry is on")

// setupScreen is where parents set the volume and start the piano.
func (a *app) setupScreen(cx *core.Scope) core.Node {
	cx.SetTheme(setupTheme())
	colors := ui.ColorsOf(cx)
	tn := func() *tones { return tonesOf(cx) }
	computer := a.kiosk.Supported()
	startErr := core.NewSignal[error](cx, nil)
	permission := core.NewSignal(cx, core.PermissionUnavailable)
	systemVolume := core.NewSignal(cx, -1.0) // -1: unknown

	// What changes outside the app is read again every second: the
	// Accessibility permission and the computer's volume.
	if computer {
		var poll func()
		poll = func() {
			cx.PermissionStatus(core.PermissionAccessibility, permission.Set)
			if v, ok := a.kiosk.SystemVolume(); ok {
				systemVolume.Set(float64(v))
			} else {
				systemVolume.Set(-1)
			}
			cx.AfterFunc(time.Second, poll)
		}
		poll()
	}
	needsPermission := func() bool {
		p := permission.Get()
		return computer && p != core.PermissionGranted && p != core.PermissionUnavailable
	}

	text := func(fn func() string, style core.Style, color func() core.Color) core.Node {
		return core.Text{ContentFunc: fn, StyleFunc: func() core.Style {
			s := ui.TypeBody.Merge(style)
			s.Color = color()
			return s
		}}
	}
	note := func(fn func() string) core.Node {
		return text(fn, core.Style{FontSize: 14, LineHeight: 20}, func() core.Color { return tn().muted })
	}
	shown := func(when func() bool, node core.Node) core.Node {
		return core.Show{When: when, Then: func(*core.Scope) core.Node { return node }}
	}

	statusText := func() string {
		switch err := a.soundErr.Get(); {
		case err != nil:
			return locales.SoundFailed(cx, err.Error())
		case a.piano.Get() == nil:
			return locales.SoundLoading(cx)
		}
		switch err := startErr.Get(); {
		case errors.Is(err, errSecureInput):
			return locales.SecureInput(cx)
		case err != nil:
			return locales.LockFailed(cx)
		}
		if needsPermission() {
			return locales.PermissionMissing(cx)
		}
		if computer && permission.Get() == core.PermissionGranted {
			return locales.PermissionGranted(cx)
		}
		return ""
	}
	// status is a soft colored box: red when something failed, amber
	// while a permission is missing, green when all is ready.
	status := shown(func() bool { return statusText() != "" }, core.View{
		StyleFunc: func() core.Style {
			c := colors.Up()
			switch {
			case a.soundErr.Get() != nil || startErr.Get() != nil:
				c = colors.Danger()
			case needsPermission():
				c = colors.Warn()
			case a.piano.Get() == nil:
				c = tn().muted
			}
			return core.Style{Direction: core.Row, Gap: 10, AlignItems: core.AlignStart, Padding: core.Symmetric(12, 16),
				CornerRadius: 16, Background: ui.Alpha(c, 0.12)}
		},
		Children: []core.Node{core.Text{
			ContentFunc: statusText,
			StyleFunc: func() core.Style {
				s := ui.TypeBody.Merge(core.Style{FontSize: 14, LineHeight: 20, FontWeight: ui.Semibold, FlexShrink: 1})
				switch {
				case a.soundErr.Get() != nil || startErr.Get() != nil:
					s.Color = colors.Danger()
				case needsPermission():
					s.Color = colors.Warn()
				case a.piano.Get() == nil:
					s.Color = tn().muted
				default:
					s.Color = colors.Up()
				}
				return s
			},
			TestID: "status",
		}},
	})

	heading := func(s string) core.Node {
		return core.Text{Content: s, StyleFunc: inked(cx, core.Style{FontSize: 17, LineHeight: 22, FontWeight: ui.Bold})}
	}
	volume := []core.Node{
		heading(locales.Volume(cx)),
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 16}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1}, Children: []core.Node{
				ui.Slider{
					Value: a.settings.volume.Get,
					OnChange: func(v float64) {
						_ = a.settings.volume.Set(v)
						if p := a.piano.GetUntracked(); p != nil {
							p.SetVolume(float32(v))
						}
					},
					Min: 0.05, Max: 0.9, Label: locales.Volume(cx), TestID: "volume",
				},
			}},
			ui.Button{Text: locales.TryIt(cx), Icon: func() *core.SVG { return iconSpeaker }, Variant: ui.ButtonOutline, TestID: "try",
				Style: core.Style{CornerRadius: 999}, OnPress: func() {
					for i, n := range []int{15, 17, 19, 22} { // C4 E4 A4 E5
						cx.AfterFunc(time.Duration(i)*250*time.Millisecond, func() { a.play(n) })
					}
				}},
		}},
	}

	intro := locales.TouchIntro(cx)
	if computer {
		intro = locales.DesktopIntro(cx)
	}
	header := core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 14, Padding: core.Edges{Bottom: 4}}, Children: []core.Node{
		logo(cx),
		core.Text{Content: locales.Title(cx), StyleFunc: func() core.Style {
			return ui.TypeHeading.Merge(core.Style{FontSize: 34, LineHeight: 40, FontWeight: ui.Bold, Color: tn().ink, TextAlign: core.TextAlignCenter})
		}},
		core.Text{Content: intro, StyleFunc: hushed(cx, core.Style{FontSize: 16, LineHeight: 24, TextAlign: core.TextAlignCenter, MaxWidth: core.Pt(480)})},
	}}

	var card []core.Node
	card = append(card, volume...)
	if computer {
		card = append(card,
			note(func() string { return locales.VolumeLocked(cx) }),
			note(func() string {
				v := systemVolume.Get()
				capped := int(kiosk.MaxSystemVolume * 100)
				switch percent := int(math.Round(v * 100)); {
				case v < 0:
					return locales.SystemVolumeUnknown(cx)
				case v > kiosk.MaxSystemVolume:
					return locales.SystemVolumeOver(cx, percent, capped)
				default:
					return locales.SystemVolumeOk(cx, percent, capped)
				}
			}),
		)
	}
	card = append(card, divider(cx), heading(locales.Scale(cx)), scaleChoice(cx, a.settings.scale), divider(cx))
	if computer {
		card = append(card, toggleRow(cx, iconMouse, accentBlue, locales.LockMouseTitle(cx), locales.LockMouseNote(cx), "lockMouse", a.settings.lockMouse))
	}
	card = append(card, toggleRow(cx, iconFirework, accentCoral, locales.VisualsTitle(cx), locales.VisualsNote(cx), "visuals", a.settings.visuals))

	// tips are how parents keep the baby in and how they stop.
	var tips []core.Node
	if computer {
		tips = append(tips, tip(cx, iconWave, accentOrange, func() string {
			if a.settings.lockMouse.Get() {
				return locales.ExitHowTyped(cx)
			}
			return locales.ExitHow(cx)
		}))
		if cx.Env().OS() == core.OSWindows {
			tips = append(tips, tip(cx, iconKeyboard, accentBlue, func() string { return locales.WindowsLimits(cx) }))
		}
	} else {
		lock := locales.TouchLockAndroid(cx)
		if cx.Env().OS() == core.OSIOS {
			lock = locales.TouchLockIOS(cx)
		}
		tips = append(tips,
			tip(cx, iconLock, accentGreen, func() string { return lock }),
			tip(cx, iconWave, accentOrange, func() string { return locales.CornerHint(cx) }),
			desktopTip(cx),
		)
	}

	startText := locales.Start(cx)
	if !computer {
		startText = locales.TouchStart(cx)
	}
	permissionButton := shown(needsPermission, ui.Button{Text: locales.OpenSettings(cx), Variant: ui.ButtonOutline, Size: ui.SizeLarge, TestID: "settings",
		Style: core.Style{CornerRadius: 999, AlignSelf: core.AlignCenter}, OnPress: func() {
			desktop.GuidePermission(cx, core.PermissionAccessibility, desktop.GuideOptions{}, permission.Set)
		}})
	settingsCard := core.View{StyleFunc: func() core.Style {
		return core.Style{Gap: 16, Padding: core.All(24), CornerRadius: 28, Background: tn().card,
			Shadows: []core.BoxShadow{{Offset: core.Point{Y: 12}, Blur: 32, Color: tn().cardShadow}}}
	}, Children: card}
	tipList := core.View{Style: core.Style{Gap: 10}, Children: tips}
	kb := keyboardStatus(cx, computer, a.keyboard.Get)
	start := startButton(startText,
		func() bool { return a.piano.Get() == nil || needsPermission() },
		func() { startErr.Set(a.start()) })

	// The page's size: wide and not tall, as a tablet or phone held
	// sideways, it lays out in two columns, the name and the start button
	// beside the settings, so it all fits without scrolling.
	page := core.NewSignal(cx, core.Size{})
	wide := func() bool { s := page.Get(); return s.Width >= 760 && s.Width > s.Height }
	// One set of views in both shapes, only the containers turn, so when the
	// screen turns everything glides to its new place instead of the two
	// layouts fading through each other. The start button closes the left
	// column sideways and the page upright.
	startWide := core.Show{When: wide, Then: func(*core.Scope) core.Node { return start }}
	startTall := core.Show{When: func() bool { return !wide() }, Then: func(*core.Scope) core.Node { return start }}
	side := func(grow float32, gap core.Points) func() core.Style {
		return func() core.Style {
			if wide() {
				return core.Style{FlexGrow: grow, FlexShrink: 1, FlexBasis: core.Pt(0), Gap: gap}
			}
			return core.Style{Gap: 20}
		}
	}
	layout := core.View{
		StyleFunc: func() core.Style {
			if wide() {
				return core.Style{Width: core.Pct(100), MaxWidth: core.Pt(1080), Direction: core.Row, Gap: 36, AlignItems: core.AlignCenter}
			}
			return core.Style{Width: core.Pct(100), MaxWidth: core.Pt(560), Gap: 20}
		},
		Children: []core.Node{
			core.View{StyleFunc: side(1, 20), Children: []core.Node{header, status, kb, permissionButton, startWide}},
			core.View{StyleFunc: side(1.15, 16), Children: []core.Node{settingsCard, tipList, startTall}},
		}}

	return core.View{
		// The page is the icon's warm paper in light appearance and a
		// plum night in dark, following the system's as it changes.
		// It takes the window's height and no more, so the settings scroll
		// inside it when they are taller.
		StyleFunc: func() core.Style {
			t := tn()
			return core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), Overflow: core.Hidden,
				Background: t.paperTop, Gradients: []core.Gradient{core.LinearGradient(180, t.paperTop, t.paperBottom)}}
		},
		OnLayout: func(b core.Bounds) { animateTurn(cx, page.GetUntracked(), b.Size); page.Set(b.Size) },
		Children: []core.Node{core.Scroll{
			// A phone or tablet scrolls under the finger with no bar, as
			// its own apps' pages do; a computer keeps its bar.
			HideIndicator: !computer,
			Style:         core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0)},
			// Centered when it fits, scrolling when it does not: at least
			// the page's height, never pinned to it.
			Children: []core.Node{core.View{
				Style: core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
					Padding: core.Symmetric(32, 20), PaddingEnv: core.EnvAll(core.EnvSafeArea)},
				Children: []core.Node{core.View{
					StyleFunc: func() core.Style {
						return core.Style{Width: core.Pct(100), MinHeight: core.Pt(max(page.Get().Height-64, 0)), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
					},
					Children: []core.Node{layout},
				}},
			}},
		}},
	}
}

// The setup's colors, from the app's icon: warm paper, dark plum ink and
// the keys' rainbow in light appearance; in dark, the ink becomes the
// night, a deep plum, and the paper the text.
var (
	coral     = core.RGB(0xf0645a)
	keyColors = []core.Color{core.RGB(0xff8a80), core.RGB(0xffb74d), core.RGB(0xffe066), core.RGB(0x7edc9a), core.RGB(0x6ec6ff)}
)

// tones are the setup's colors in one appearance.
type tones struct {
	paperTop, paperBottom core.Color // the page, top to bottom
	card, cardShadow      core.Color // the settings card
	ink, muted            core.Color // text
	panel                 core.Color // the soft boxes on the page: tips, the keyboard, the logo
	tile, tileEdge        core.Color // a scale not chosen
	chosen                core.Color // the chosen scale's tint
	line, track, knob     core.Color // dividers; a switch's track when off and its knob
	green                 core.Color // a keyboard attached
}

var (
	lightTones = tones{
		paperTop: core.RGB(0xfff4dc), paperBottom: core.RGB(0xffd9b8),
		card: core.RGB(0xffffff), cardShadow: core.RGBA(0x8a4b2a, 0.14),
		ink: core.RGB(0x3b3355), muted: core.RGB(0x6e6585),
		panel: core.RGBA(0xffffff, 0.55),
		tile:  core.RGB(0xfffaf4), tileEdge: core.RGBA(0x3b3355, 0.08), chosen: core.RGBA(0xf0645a, 0.08),
		line: core.RGBA(0x3b3355, 0.08), track: core.RGBA(0x3b3355, 0.15), knob: core.RGB(0xffffff),
		green: core.RGB(0x2f8f5f),
	}
	darkTones = tones{
		paperTop: core.RGB(0x2e2540), paperBottom: core.RGB(0x1c1628),
		card: core.RGB(0x352c48), cardShadow: core.RGBA(0x000000, 0.35),
		ink: core.RGB(0xfbf1e4), muted: core.RGB(0xc0b6d2),
		panel: core.RGBA(0xffffff, 0.07),
		tile:  core.RGB(0x3e3453), tileEdge: core.RGBA(0xffffff, 0.08), chosen: core.RGBA(0xf0645a, 0.18),
		line: core.RGBA(0xffffff, 0.10), track: core.RGBA(0xffffff, 0.22), knob: core.RGB(0xffffff),
		green: core.RGB(0x74d6a2),
	}
)

// tonesOf is the tones of the appearance cx shows; a binding that reads
// it runs again when the system turns light or dark.
func tonesOf(cx *core.Scope) *tones {
	if cx.Dark() {
		return &darkTones
	}
	return &lightTones
}

// inked and hushed are text styles in the ink and the muted ink of the
// appearance cx shows.
func inked(cx *core.Scope, s core.Style) func() core.Style {
	return func() core.Style { s := s; s.Color = tonesOf(cx).ink; return ui.TypeBody.Merge(s) }
}

func hushed(cx *core.Scope, s core.Style) func() core.Style {
	return func() core.Style { s := s; s.Color = tonesOf(cx).muted; return ui.TypeBody.Merge(s) }
}

// setupTheme is the default theme with the icon's ink and a coral accent,
// light and dark, so the controls match the page.
func setupTheme() *core.Theme {
	t := core.DefaultTheme().Clone()
	for _, x := range []struct {
		p    *core.Palette
		tn   tones
		dark bool
	}{{&t.Light, lightTones, false}, {&t.Dark, darkTones, true}} {
		p, tn := x.p, x.tn
		p.Background, p.Surface = tn.card, tn.card
		p.Text, p.TextMuted = tn.ink, tn.muted
		p.Border = tn.ink.Faded(0.18)
		p.SurfaceHover, p.SurfacePressed = tn.ink.Faded(0.08), tn.ink.Faded(0.14)
		p.Accent, p.AccentHover, p.AccentPressed = coral, core.RGB(0xe5574d), core.RGB(0xd44b42)
		p.Selection, p.Focus = core.RGBA(0xf0645a, 0.22), core.RGBA(0xf0645a, 0.55)
		p.Success, p.Warning = tn.green, core.RGB(0xb45309)
		if x.dark {
			p.AccentHover, p.AccentPressed = core.RGB(0xf47a70), core.RGB(0xe5574d)
			p.Warning, p.Danger = core.RGB(0xf5b84f), core.RGB(0xff8f86)
			p.Scrollbar = tn.ink.Faded(0.35)
		}
		p.SetColor("accentFill", coral)
		p.SetColor("fieldBorder", p.Border)
		p.SetColor("track", tn.track)
		p.SetColor("knob", tn.knob)
	}
	return t
}

// logo is the icon's five colored keys, small.
func logo(cx *core.Scope) core.Node {
	var keys []core.Node
	for _, c := range keyColors {
		keys = append(keys, core.View{Style: core.Style{Width: core.Pt(18), Height: core.Pt(56), CornerRadius: 7, Background: c,
			Gradients: []core.Gradient{core.LinearGradient(180, core.RGBA(0xffffff, 0.35), core.RGBA(0xffffff, 0), core.RGBA(0x000000, 0.08))}}})
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Direction: core.Row, Gap: 5, Padding: core.All(12), CornerRadius: 24, Background: tonesOf(cx).panel}
	}, Children: keys}
}

// scaleChoice is three tiles, one per scale, each with a row of colored
// keys as many as the scale has notes to an octave: the chosen one lifts
// in the accent's tint and edge.
func scaleChoice(cx *core.Scope, scale *core.Persisted[piano.Scale]) core.Node {
	names := map[piano.Scale][2]string{
		piano.Pentatonic: {locales.ScalePentatonic(cx), locales.ScalePentatonicNote(cx)},
		piano.Major:      {locales.ScaleMajor(cx), locales.ScaleMajorNote(cx)},
		piano.Chromatic:  {locales.ScaleChromatic(cx), locales.ScaleChromaticNote(cx)},
	}
	var tiles []core.Node
	for _, s := range piano.Scales {
		chosen := func() bool { return scale.Get() == s }
		var bars []core.Node
		for c := range s.PerOctave() {
			h := float64(c) / float64(s.PerOctave()) * 330
			l := 0.64
			if s == piano.Chromatic && sharp(piano.Chromatic.Pitch(c)) {
				l = 0.51
			}
			bars = append(bars, core.View{Style: core.Style{FlexGrow: 1, Height: core.Pt(22), CornerRadius: 3, Background: hsl(h, 0.8, l)}})
		}
		tiles = append(tiles, base.Button{
			Role: core.RoleRadio, Selected: chosen, Label: names[s][0], TestID: "scale-" + string(s),
			OnPress: func(core.PressEvent) { _ = scale.Set(s) },
			StyleFunc: func() core.Style {
				t := tonesOf(cx)
				st := core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0), Gap: 8, Padding: core.All(12), CornerRadius: 18,
					BorderWidth: 2, BorderColor: t.tileEdge, Background: t.tile,
					Active: &core.Style{Opacity: core.Alpha(0.8)}}
				if chosen() {
					st.BorderColor = coral
					st.Background = t.chosen
				}
				return st
			},
			Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, Gap: 2}, Children: bars},
				core.Text{Content: names[s][0], StyleFunc: inked(cx, core.Style{FontSize: 15, LineHeight: 20, FontWeight: ui.Bold})},
				core.Text{Content: names[s][1], StyleFunc: hushed(cx, core.Style{FontSize: 12, LineHeight: 16})},
			},
		})
	}
	return core.View{Style: core.Style{Direction: core.Row, Gap: 10}, Children: tiles}
}

// toggleRow is a setting the parents turn on or off: an icon in a tinted
// square, a title and what it does, and a switch; the whole row presses.
func toggleRow(cx *core.Scope, svg *core.SVG, tint accent, title, note, id string, on *core.Persisted[bool]) core.Node {
	return base.Button{
		Role: core.RoleSwitch, Selected: on.Get, Label: title, TestID: id,
		OnPress: func(core.PressEvent) { _ = on.Set(!on.Get()) },
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 14, Padding: core.Symmetric(4, 0),
			Active: &core.Style{Opacity: core.Alpha(0.8)}},
		Children: []core.Node{
			badge(cx, svg, tint, 44, 24, 14),
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
				core.Text{Content: title, StyleFunc: inked(cx, core.Style{FontSize: 15, LineHeight: 20, FontWeight: ui.Semibold})},
				core.Text{Content: note, StyleFunc: hushed(cx, core.Style{FontSize: 13, LineHeight: 18})},
			}},
			// The row takes the press; the switch shows the state.
			core.View{Style: core.Style{Width: core.Pt(50), Height: core.Pt(30), CornerRadius: 15, Padding: core.All(3)},
				StyleFunc: func() core.Style {
					st := core.Style{Width: core.Pt(50), Height: core.Pt(30), CornerRadius: 15, Padding: core.All(3), Background: tonesOf(cx).track, AlignItems: core.AlignStart}
					if on.Get() {
						st.Background, st.AlignItems = coral, core.AlignEnd
					}
					return st
				},
				Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(24), Height: core.Pt(24), CornerRadius: 12, Background: core.RGB(0xffffff),
					Shadows: []core.BoxShadow{{Offset: core.Point{Y: 1}, Blur: 3, Color: core.RGBA(0x000000, 0.2)}}}}}},
		},
	}
}

// keyboardStatus says, on a phone or tablet, whether a keyboard is attached
// and so what the baby will play: the keyboard, or the big keys on the
// screen. The system tells, or a key pressed here shows it, should the
// system not; computers always play their keyboard.
func keyboardStatus(cx *core.Scope, computer bool, attached func() bool) core.Node {
	if computer {
		return nil
	}
	typed := core.NewSignal(cx, false)
	cx.Effect(func() {
		if !attached() {
			typed.Set(false)
		}
	})
	cx.Effect(func() {
		if len(core.KeysDown(cx)) > 0 {
			cx.Untrack(func() { typed.Set(true) })
		}
	})
	on := func() bool { return attached() || typed.Get() }
	return core.View{
		TestID: "keyboard",
		StyleFunc: func() core.Style {
			t := tonesOf(cx)
			st := core.Style{Direction: core.Row, Gap: 14, AlignItems: core.AlignCenter, Padding: core.Symmetric(14, 18),
				CornerRadius: 18, Background: t.panel, BorderWidth: 2, BorderColor: core.RGBA(0xffffff, 0)}
			if on() {
				st.Background, st.BorderColor = ui.Alpha(t.green, 0.12), ui.Alpha(t.green, 0.4)
			}
			return st
		},
		Children: []core.Node{
			core.Icon{SVGFunc: func() *core.SVG {
				if on() {
					return iconKeyboard
				}
				return iconTap
			}, StyleFunc: func() core.Style {
				c := tonesOf(cx).muted
				if on() {
					c = tonesOf(cx).green
				}
				return core.Style{Width: core.Pt(28), Height: core.Pt(28), FlexShrink: 0, Color: c}
			}},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
				core.Text{ContentFunc: func() string {
					if on() {
						return locales.KeyboardOn(cx)
					}
					return locales.KeyboardOff(cx)
				}, StyleFunc: func() core.Style {
					c := tonesOf(cx).ink
					if on() {
						c = tonesOf(cx).green
					}
					return ui.TypeBody.Merge(core.Style{FontSize: 15, LineHeight: 20, FontWeight: ui.Bold, Color: c})
				}},
				core.Text{ContentFunc: func() string {
					if on() {
						return locales.KeyboardOnNote(cx)
					}
					return locales.KeyboardOffNote(cx)
				}, StyleFunc: hushed(cx, core.Style{FontSize: 13, LineHeight: 18})},
			}},
		},
	}
}

// divider is a hairline across the card.
func divider(cx *core.Scope) core.Node {
	return core.View{StyleFunc: func() core.Style { return core.Style{Height: core.Pt(1), Background: tonesOf(cx).line} }}
}

// tip is a line of advice for parents beside an icon.
func tip(cx *core.Scope, svg *core.SVG, tint accent, fn func() string) core.Node {
	return core.View{StyleFunc: panelStyle(cx),
		Children: []core.Node{
			badge(cx, svg, tint, 32, 20, 10),
			core.Text{ContentFunc: fn, StyleFunc: inked(cx, core.Style{FontSize: 15, LineHeight: 22, FlexShrink: 1, FlexGrow: 1})},
		}}
}

// panelStyle is a tip's box: a soft panel on the page, the icon beside
// the words.
func panelStyle(cx *core.Scope) func() core.Style {
	return func() core.Style {
		return core.Style{Direction: core.Row, Gap: 14, AlignItems: core.AlignCenter, Padding: core.Symmetric(14, 18),
			CornerRadius: 18, Background: tonesOf(cx).panel}
	}
}

// desktopURL is the piano's homepage, where parents download the Mac and
// Windows apps.
const desktopURL = "https://mashpiano.herosizy.dev"

// desktopTip tells parents on a phone or tablet that the piano is also a
// computer app, with a link to download it; it sits with the tips on the
// setup page, away from the baby's keys.
func desktopTip(cx *core.Scope) core.Node {
	return core.View{StyleFunc: panelStyle(cx),
		Children: []core.Node{
			badge(cx, iconLaptop, accentPurple, 32, 20, 10),
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 6}, Children: []core.Node{
				core.Text{Content: locales.DesktopApp(cx), StyleFunc: inked(cx, core.Style{FontSize: 15, LineHeight: 22})},
				ui.Link{Text: locales.DesktopAppLink(cx) + " ›", Href: desktopURL, TestID: "desktop",
					Style: core.Style{FontSize: 15, LineHeight: 22, FontWeight: ui.Semibold}},
			}},
		}}
}

// startButton is the big rounded button that starts the piano, a warm
// gradient of the icon's first keys.
func startButton(label string, disabled func() bool, press func()) core.Node {
	return base.Button{
		Label: label, TestID: "start", Disabled: disabled, OnPress: func(core.PressEvent) { press() },
		Style: core.Style{Height: core.Pt(64), CornerRadius: 32, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			Background: coral, Gradients: []core.Gradient{core.LinearGradient(90, core.RGB(0xf26b6b), core.RGB(0xf59a3c))},
			Shadows:  []core.BoxShadow{{Offset: core.Point{Y: 10}, Blur: 24, Color: core.RGBA(0xf0645a, 0.35)}},
			Active:   &core.Style{Opacity: core.Alpha(0.85)},
			Disabled: &core.Style{Opacity: core.Alpha(0.45)}},
		Children: []core.Node{core.Text{Content: label, Style: ui.TypeBody.Merge(core.Style{FontSize: 20, LineHeight: 26, FontWeight: ui.Bold, Color: core.RGB(0xffffff)})}},
	}
}
