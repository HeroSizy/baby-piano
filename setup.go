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
	cx.SetSubtreeAppearance(core.AppearanceLight)
	colors := ui.ColorsOf(cx)
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
		return text(fn, core.Style{FontSize: 14, LineHeight: 20}, ui.Fixed(muted))
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
				c = muted
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
					s.Color = muted
				default:
					s.Color = colors.Up()
				}
				return s
			},
			TestID: "status",
		}},
	})

	heading := func(s string) core.Node {
		return core.Text{Content: s, Style: ui.TypeBody.Merge(core.Style{FontSize: 17, LineHeight: 22, FontWeight: ui.Bold, Color: ink})}
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
			ui.Button{Text: "🔊 " + locales.TryIt(cx), Variant: ui.ButtonOutline, TestID: "try",
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
		logo(),
		core.Text{Content: locales.Title(cx), Style: ui.TypeHeading.Merge(core.Style{FontSize: 34, LineHeight: 40, FontWeight: ui.Bold, Color: ink, TextAlign: core.TextAlignCenter})},
		core.Text{Content: intro, Style: ui.TypeBody.Merge(core.Style{FontSize: 16, LineHeight: 24, Color: muted, TextAlign: core.TextAlignCenter, MaxWidth: core.Pt(480)})},
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
	card = append(card, divider(), heading(locales.Scale(cx)), scaleChoice(cx, a.settings.scale), divider())
	if computer {
		card = append(card, toggleRow("🖱️", locales.LockMouseTitle(cx), locales.LockMouseNote(cx), "lockMouse", a.settings.lockMouse))
	}
	card = append(card, toggleRow("🎆", locales.VisualsTitle(cx), locales.VisualsNote(cx), "visuals", a.settings.visuals))

	// tips are how parents keep the baby in and how they stop.
	var tips []core.Node
	if computer {
		tips = append(tips, tip("👋", func() string {
			if a.settings.lockMouse.Get() {
				return locales.ExitHowTyped(cx)
			}
			return locales.ExitHow(cx)
		}))
		if cx.Env().OS() == core.OSWindows {
			tips = append(tips, tip("⌨️", func() string { return locales.WindowsLimits(cx) }))
		}
	} else {
		lock := locales.TouchLockAndroid(cx)
		if cx.Env().OS() == core.OSIOS {
			lock = locales.TouchLockIOS(cx)
		}
		tips = append(tips,
			tip("🔒", func() string { return lock }),
			tip("👋", func() string { return locales.CornerHint(cx) }),
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
	settingsCard := core.View{Style: core.Style{Gap: 16, Padding: core.All(24), CornerRadius: 28, Background: core.RGB(0xffffff),
		Shadows: []core.BoxShadow{{Offset: core.Point{Y: 12}, Blur: 32, Color: core.RGBA(0x8a4b2a, 0.14)}}},
		Children: card}
	tipList := core.View{Style: core.Style{Gap: 10}, Children: tips}
	kb := keyboardStatus(cx, computer)
	start := startButton(startText,
		func() bool { return a.piano.Get() == nil || needsPermission() },
		func() { startErr.Set(a.start()) })

	// The page's size: wide and not tall, as a tablet or phone held
	// sideways, it lays out in two columns, the name and the start button
	// beside the settings, so it all fits without scrolling.
	page := core.NewSignal(cx, core.Size{})
	wide := func() bool { s := page.Get(); return s.Width >= 760 && s.Width > s.Height }
	column := func(*core.Scope) core.Node {
		return core.View{Style: core.Style{Width: core.Pct(100), MaxWidth: core.Pt(560), Gap: 20},
			Children: []core.Node{header, status, kb, permissionButton, settingsCard, tipList, start}}
	}
	columns := func(*core.Scope) core.Node {
		return core.View{Style: core.Style{Width: core.Pct(100), MaxWidth: core.Pt(1080), Direction: core.Row, Gap: 36, AlignItems: core.AlignCenter},
			Children: []core.Node{
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), Gap: 20},
					Children: []core.Node{header, status, kb, permissionButton, start}},
				core.View{Style: core.Style{FlexGrow: 1.15, FlexShrink: 1, FlexBasis: core.Pt(0), Gap: 16},
					Children: []core.Node{settingsCard, tipList}},
			}}
	}

	return core.View{
		// The page is the icon's warm paper whatever the system's
		// appearance: dark gray on black is hard to read at a glance.
		// It takes the window's height and no more, so the settings scroll
		// inside it when they are taller.
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), Overflow: core.Hidden,
			Background: paperTop, Gradients: []core.Gradient{core.LinearGradient(180, paperTop, paperBottom)}},
		OnLayout: func(b core.Bounds) { page.Set(b.Size) },
		Children: []core.Node{ui.ScrollView{
			EdgeColor: ui.Fixed(core.RGBA(0xffffff, 0)),
			Style:     core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0)},
			// Centered when it fits, scrolling when it does not: at least
			// the page's height, never pinned to it.
			ContentStyle: core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				Padding: core.Symmetric(32, 20), PaddingEnv: core.EnvAll(core.EnvSafeArea)},
			Children: []core.Node{core.View{
				StyleFunc: func() core.Style {
					return core.Style{Width: core.Pct(100), MinHeight: core.Pt(max(page.Get().Height-64, 0)), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
				},
				Children: []core.Node{core.Show{When: wide, Then: columns, Else: column}},
			}},
		}},
	}
}

// The setup's colors, from the app's icon: warm paper, dark plum ink and
// the keys' rainbow.
var (
	paperTop    = core.RGB(0xfff4dc)
	paperBottom = core.RGB(0xffd9b8)
	ink         = core.RGB(0x3b3355)
	muted       = core.RGB(0x6e6585)
	coral       = core.RGB(0xf0645a)
	keyColors   = []core.Color{core.RGB(0xff8a80), core.RGB(0xffb74d), core.RGB(0xffe066), core.RGB(0x7edc9a), core.RGB(0x6ec6ff)}
)

// setupTheme is the default theme, light, with the icon's ink and a coral
// accent, so the controls match the page.
func setupTheme() *core.Theme {
	t := core.DefaultTheme().Clone()
	p := &t.Light
	p.Background, p.Surface = core.RGB(0xffffff), core.RGB(0xffffff)
	p.Text, p.TextMuted = ink, muted
	p.Border = ink.Faded(0.18)
	p.Accent, p.AccentHover, p.AccentPressed = coral, core.RGB(0xe5574d), core.RGB(0xd44b42)
	p.Selection, p.Focus = core.RGBA(0xf0645a, 0.22), core.RGBA(0xf0645a, 0.55)
	p.Success, p.Warning = core.RGB(0x2f8f5f), core.RGB(0xb45309)
	p.SetColor("accentFill", coral)
	p.SetColor("fieldBorder", p.Border)
	p.SetColor("track", ink.Faded(0.15))
	t.Dark = t.Light
	return t
}

// logo is the icon's five colored keys, small.
func logo() core.Node {
	var keys []core.Node
	for _, c := range keyColors {
		keys = append(keys, core.View{Style: core.Style{Width: core.Pt(18), Height: core.Pt(56), CornerRadius: 7, Background: c,
			Gradients: []core.Gradient{core.LinearGradient(180, core.RGBA(0xffffff, 0.35), core.RGBA(0xffffff, 0), core.RGBA(0x000000, 0.08))}}})
	}
	return core.View{Style: core.Style{Direction: core.Row, Gap: 5, Padding: core.All(12), CornerRadius: 24, Background: core.RGBA(0xffffff, 0.6)},
		Children: keys}
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
				st := core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0), Gap: 8, Padding: core.All(12), CornerRadius: 18,
					BorderWidth: 2, BorderColor: ink.Faded(0.08), Background: core.RGB(0xfffaf4),
					Active: &core.Style{Opacity: core.Alpha(0.8)}}
				if chosen() {
					st.BorderColor = coral
					st.Background = core.RGBA(0xf0645a, 0.08)
				}
				return st
			},
			Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, Gap: 2}, Children: bars},
				core.Text{Content: names[s][0], Style: ui.TypeBody.Merge(core.Style{FontSize: 15, LineHeight: 20, FontWeight: ui.Bold, Color: ink})},
				core.Text{Content: names[s][1], Style: ui.TypeBody.Merge(core.Style{FontSize: 12, LineHeight: 16, Color: muted})},
			},
		})
	}
	return core.View{Style: core.Style{Direction: core.Row, Gap: 10}, Children: tiles}
}

// toggleRow is a setting the parents turn on or off: an emoji in a tinted
// square, a title and what it does, and a switch; the whole row presses.
func toggleRow(emoji, title, note, id string, on *core.Persisted[bool]) core.Node {
	return base.Button{
		Role: core.RoleSwitch, Selected: on.Get, Label: title, TestID: id,
		OnPress: func(core.PressEvent) { _ = on.Set(!on.Get()) },
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 14, Padding: core.Symmetric(4, 0),
			Active: &core.Style{Opacity: core.Alpha(0.8)}},
		Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(44), Height: core.Pt(44), CornerRadius: 14, Background: core.RGB(0xfff1e4),
				AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				Children: []core.Node{core.Text{Content: emoji, Style: ui.TypeBody.Merge(core.Style{FontSize: 22, LineHeight: 26})}}},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
				core.Text{Content: title, Style: ui.TypeBody.Merge(core.Style{FontSize: 15, LineHeight: 20, FontWeight: ui.Semibold, Color: ink})},
				core.Text{Content: note, Style: ui.TypeBody.Merge(core.Style{FontSize: 13, LineHeight: 18, Color: muted})},
			}},
			// The row takes the press; the switch shows the state.
			core.View{Style: core.Style{Width: core.Pt(50), Height: core.Pt(30), CornerRadius: 15, Padding: core.All(3)},
				StyleFunc: func() core.Style {
					st := core.Style{Width: core.Pt(50), Height: core.Pt(30), CornerRadius: 15, Padding: core.All(3), Background: ink.Faded(0.15), AlignItems: core.AlignStart}
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
func keyboardStatus(cx *core.Scope, computer bool) core.Node {
	if computer {
		return nil
	}
	typed := core.NewSignal(cx, false)
	cx.Effect(func() {
		if !cx.Env().HardwareKeyboard() {
			typed.Set(false)
		}
	})
	cx.Effect(func() {
		if len(core.KeysDown(cx)) > 0 {
			cx.Untrack(func() { typed.Set(true) })
		}
	})
	on := func() bool { return cx.Env().HardwareKeyboard() || typed.Get() }
	return core.View{
		TestID: "keyboard",
		StyleFunc: func() core.Style {
			st := core.Style{Direction: core.Row, Gap: 14, AlignItems: core.AlignCenter, Padding: core.Symmetric(14, 18),
				CornerRadius: 18, Background: core.RGBA(0xffffff, 0.55), BorderWidth: 2, BorderColor: core.RGBA(0xffffff, 0)}
			if on() {
				st.Background, st.BorderColor = core.RGBA(0x2f8f5f, 0.10), core.RGBA(0x2f8f5f, 0.35)
			}
			return st
		},
		Children: []core.Node{
			core.Text{ContentFunc: func() string {
				if on() {
					return "⌨️"
				}
				return "👆"
			}, Style: ui.TypeBody.Merge(core.Style{FontSize: 24, LineHeight: 28})},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
				core.Text{ContentFunc: func() string {
					if on() {
						return locales.KeyboardOn(cx)
					}
					return locales.KeyboardOff(cx)
				}, StyleFunc: func() core.Style {
					c := ink
					if on() {
						c = core.RGB(0x2f8f5f)
					}
					return ui.TypeBody.Merge(core.Style{FontSize: 15, LineHeight: 20, FontWeight: ui.Bold, Color: c})
				}},
				core.Text{ContentFunc: func() string {
					if on() {
						return locales.KeyboardOnNote(cx)
					}
					return locales.KeyboardOffNote(cx)
				}, Style: ui.TypeBody.Merge(core.Style{FontSize: 13, LineHeight: 18, Color: muted})},
			}},
		},
	}
}

// divider is a hairline across the card.
func divider() core.Node {
	return core.View{Style: core.Style{Height: core.Pt(1), Background: ink.Faded(0.08)}}
}

// tip is a line of advice for parents beside an emoji.
func tip(emoji string, fn func() string) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Gap: 12, AlignItems: core.AlignStart, Padding: core.Symmetric(14, 18),
		CornerRadius: 18, Background: core.RGBA(0xffffff, 0.55)},
		Children: []core.Node{
			core.Text{Content: emoji, Style: ui.TypeBody.Merge(core.Style{FontSize: 18, LineHeight: 22})},
			core.Text{ContentFunc: fn, Style: ui.TypeBody.Merge(core.Style{FontSize: 15, LineHeight: 22, Color: ink, FlexShrink: 1, FlexGrow: 1})},
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
