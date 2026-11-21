package main

import (
	"errors"
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/HeroSizy/baby-piano/kiosk"
	"github.com/HeroSizy/baby-piano/locales"
)

var errSecureInput = errors.New("secure keyboard entry is on")

// setupScreen is where parents set the volume and start the piano.
func (a *app) setupScreen(cx *core.Scope) core.Node {
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
	line := func(fn func() string) core.Node { return text(fn, core.Style{}, colors.Text) }
	note := func(fn func() string) core.Node { return text(fn, core.Style{}, colors.Muted) }
	shown := func(when func() bool, node core.Node) core.Node {
		return core.Show{When: when, Then: func(*core.Scope) core.Node { return node }}
	}

	status := func() core.Node {
		return core.Text{
			ContentFunc: func() string {
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
			},
			StyleFunc: func() core.Style {
				s := ui.TypeBody
				switch {
				case a.soundErr.Get() != nil || startErr.Get() != nil:
					s.Color = colors.Danger()
				case needsPermission():
					s.Color = colors.Warn()
				default:
					s.Color = colors.Up()
				}
				return s
			},
			TestID: "status",
		}
	}

	volume := core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
		note(func() string { return locales.Volume(cx) }),
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(260)}, Children: []core.Node{
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
			ui.Button{Text: locales.TryIt(cx), Variant: ui.ButtonSecondary, TestID: "try", OnPress: func() {
				for i, n := range []int{15, 17, 19, 22} { // C4 E4 A4 E5
					cx.AfterFunc(time.Duration(i)*250*time.Millisecond, func() { a.play(n) })
				}
			}},
		}},
	}}

	children := []core.Node{
		core.Text{Content: "🎹 " + locales.Title(cx), Style: ui.TypeHeading.Merge(core.Style{FontSize: 26, LineHeight: 32})},
	}
	if computer {
		children = append(children,
			note(func() string { return locales.DesktopIntro(cx) }),
			text(func() string {
				if a.settings.lockMouse.Get() {
					return locales.ExitHowTyped(cx)
				}
				return locales.ExitHow(cx)
			}, core.Style{FontWeight: ui.Semibold}, colors.Text),
			status(),
			shown(needsPermission, ui.Button{Text: locales.OpenSettings(cx), Variant: ui.ButtonOutline, TestID: "settings", OnPress: func() {
				desktop.GuidePermission(cx, core.PermissionAccessibility, desktop.GuideOptions{}, permission.Set)
			}}),
			volume,
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
		if cx.Env().OS() == core.OSWindows {
			children = append(children, note(func() string { return locales.WindowsLimits(cx) }))
		}
		children = append(children,
			ui.Checkbox{Text: locales.LockMouse(cx), Checked: a.settings.lockMouse.Get, OnChange: func(on bool) { _ = a.settings.lockMouse.Set(on) }, TestID: "lockMouse"},
		)
	} else {
		lock := locales.TouchLockAndroid(cx)
		if cx.Env().OS() == core.OSIOS {
			lock = locales.TouchLockIOS(cx)
		}
		children = append(children,
			note(func() string { return locales.TouchIntro(cx) }),
			status(),
			volume,
			line(func() string { return lock }),
		)
	}
	children = append(children,
		ui.Checkbox{Text: locales.ShowVisuals(cx), Checked: a.settings.visuals.Get, OnChange: func(on bool) { _ = a.settings.visuals.Set(on) }, TestID: "visuals"},
	)
	startText := locales.Start(cx)
	if !computer {
		startText = locales.TouchStart(cx)
	}
	children = append(children, core.View{Style: core.Style{Padding: core.Edges{Top: 8}}, Children: []core.Node{
		ui.Button{
			Text: startText, Size: ui.SizeLarge, TestID: "start",
			Disabled: func() bool { return a.piano.Get() == nil || needsPermission() },
			OnPress:  func() { startErr.Set(a.start()) },
		},
	}})

	return ui.ScrollView{Children: []core.Node{
		core.View{
			Style: core.Style{Gap: 14, Padding: core.Symmetric(24, 32), PaddingEnv: core.EnvAll(core.EnvSafeArea), MaxWidth: core.Pt(640)},
			Children: children,
		},
	}}
}
