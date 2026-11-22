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
					for i, n := range []int{36, 40, 45, 52} { // C4 E4 A4 E5, in every scale
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
		card = append(card,
			ui.Checkbox{Text: locales.LockMouse(cx), Checked: a.settings.lockMouse.Get, OnChange: func(on bool) { _ = a.settings.lockMouse.Set(on) }, TestID: "lockMouse"},
		)
	}
	card = append(card,
		ui.Checkbox{Text: locales.ShowVisuals(cx), Checked: a.settings.visuals.Get, OnChange: func(on bool) { _ = a.settings.visuals.Set(on) }, TestID: "visuals"},
	)

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
	children := []core.Node{
		header,
		status,
		shown(needsPermission, ui.Button{Text: locales.OpenSettings(cx), Variant: ui.ButtonOutline, Size: ui.SizeLarge, TestID: "settings",
			Style: core.Style{CornerRadius: 999, AlignSelf: core.AlignCenter}, OnPress: func() {
				desktop.GuidePermission(cx, core.PermissionAccessibility, desktop.GuideOptions{}, permission.Set)
			}}),
		core.View{Style: core.Style{Gap: 16, Padding: core.All(24), CornerRadius: 28, Background: core.RGB(0xffffff),
			Shadows: []core.BoxShadow{{Offset: core.Point{Y: 12}, Blur: 32, Color: core.RGBA(0x8a4b2a, 0.14)}}},
			Children: card},
		core.View{Style: core.Style{Gap: 10}, Children: tips},
		startButton(startText,
			func() bool { return a.piano.Get() == nil || needsPermission() },
			func() { startErr.Set(a.start()) }),
	}

	return core.View{
		// The page is the icon's warm paper whatever the system's
		// appearance: dark gray on black is hard to read at a glance.
		Style: core.Style{FlexGrow: 1, Background: paperTop, Gradients: []core.Gradient{core.LinearGradient(180, paperTop, paperBottom)}},
		Children: []core.Node{ui.ScrollView{
			EdgeColor: ui.Fixed(core.RGBA(0xffffff, 0)),
			Style:     core.Style{FlexGrow: 1},
			ContentStyle: core.Style{MinHeight: core.Pct(100), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				Padding: core.Symmetric(32, 20), PaddingEnv: core.EnvAll(core.EnvSafeArea)},
			Children: []core.Node{core.View{
				Style:    core.Style{Width: core.Pct(100), MaxWidth: core.Pt(560), Gap: 20},
				Children: children,
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

// scaleChoice is the parents' choice of the notes the piano plays.
func scaleChoice(cx *core.Scope, setting *core.Persisted[string]) core.Node {
	labels := map[piano.Scale]string{
		piano.Pentatonic: locales.ScalePentatonic(cx),
		piano.Major:      locales.ScaleMajor(cx),
		piano.Chromatic:  locales.ScaleChromatic(cx),
	}
	var radios []core.Node
	for _, s := range piano.Scales {
		radios = append(radios, ui.Radio{
			Text:     labels[s],
			Checked:  func() bool { return piano.ParseScale(setting.Get()) == s },
			OnChange: func(bool) { _ = setting.Set(string(s)) },
			TestID:   "scale-" + string(s),
		})
	}
	return ui.RadioGroup{Label: locales.Scale(cx), Children: radios, TestID: "scale"}
}
