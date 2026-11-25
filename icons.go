package main

import "github.com/nsgo-dev/nsgo/core"

// The setup page's icons, drawn for the piano in place of emoji: 24-point
// line drawings with round ends, each with a soft fill under its main
// shape, all in currentColor so a theme tints them as it tints text. The
// homepage's waving hand (site/public/index.html) is the same drawing.

const (
	svgHead = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">`
	svgTail = `</svg>`
)

func mustSVG(body string) *core.SVG {
	s, err := core.ParseSVG([]byte(svgHead + body + svgTail))
	if err != nil {
		panic(err)
	}
	return s
}

var (
	// iconSpeaker is a speaker with two waves: try the volume.
	iconSpeaker = mustSVG(`<path d="M4 9h2.6L11 5.2v13.6L6.6 15H4a1 1 0 0 1-1-1v-4a1 1 0 0 1 1-1z" fill="currentColor" fill-opacity=".22"/>` +
		`<path d="M15.2 9.2a4 4 0 0 1 0 5.6"/><path d="M18.2 6.4a8 8 0 0 1 0 11.2"/>`)

	// iconHeadphones is a pair of headphones: the sound plays in them.
	iconHeadphones = mustSVG(`<path d="M4 15v-3a8 8 0 0 1 16 0v3"/>` +
		`<rect x="3" y="14" width="5" height="7" rx="2" fill="currentColor" fill-opacity=".22"/>` +
		`<rect x="16" y="14" width="5" height="7" rx="2" fill="currentColor" fill-opacity=".22"/>`)

	// iconMouse is a mouse with its wheel.
	iconMouse = mustSVG(`<rect x="6" y="3" width="12" height="18" rx="6" fill="currentColor" fill-opacity=".22"/><path d="M12 7v3"/>`)

	// iconFirework is a burst: eight rays round a glowing heart, with a
	// spark beside it.
	iconFirework = mustSVG(`<circle cx="11" cy="13" r="2.4" fill="currentColor" fill-opacity=".22"/>` +
		`<path d="M11 7.5V5M11 21v-2.5M5.5 13H3M19 13h-2.5M7.1 9.1 5.3 7.3M16.7 18.7l-1.8-1.8M7.1 16.9l-1.8 1.8M16.7 7.3l-1.8 1.8"/>` +
		`<path d="M19.5 2.5v3M18 4h3" stroke-width="1.6"/>`)

	// iconWave is an open hand waving, tipped over, two marks beside it.
	// The hand is lucide's (ISC licence, which NSGo ships in icons/lucide).
	iconWave = mustSVG(`<g transform="rotate(-14 13 14) translate(1.5 1) scale(.9)">` +
		`<path d="M18 11V6a2 2 0 0 0-2-2a2 2 0 0 0-2 2"/><path d="M14 10V4a2 2 0 0 0-2-2a2 2 0 0 0-2 2v2"/>` +
		`<path d="M10 10.5V6a2 2 0 0 0-2-2a2 2 0 0 0-2 2v8"/>` +
		`<path d="M18 8a2 2 0 1 1 4 0v6a8 8 0 0 1-8 8h-2c-2.8 0-4.5-.86-5.99-2.34l-3.6-3.6a2 2 0 0 1 2.83-2.82L7 15"/></g>` +
		`<path d="M2.6 8.4a5 5 0 0 1 1.6-4.2M1.6 4.6l.9-1.6" stroke-width="1.6"/>`)

	// iconLock is a padlock.
	iconLock = mustSVG(`<rect x="4.5" y="10.5" width="15" height="10.5" rx="2.5" fill="currentColor" fill-opacity=".22"/>` +
		`<path d="M8 10.5V7.5a4 4 0 0 1 8 0v3"/><path d="M12 15v2"/>`)

	// iconKeyboard is a keyboard: two rows of keys and the space bar.
	iconKeyboard = mustSVG(`<rect x="2" y="5.5" width="20" height="13" rx="3" fill="currentColor" fill-opacity=".22"/>` +
		`<path d="M6 9.5h.01M10 9.5h.01M14 9.5h.01M18 9.5h.01M8 12.5h.01M12 12.5h.01M16 12.5h.01M8.5 15.5h7"/>`)

	// iconTap is a finger tapping, lucide's pointing hand (ISC licence).
	iconTap = mustSVG(`<path d="M22 14a8 8 0 0 1-8 8"/><path d="M18 11v-1a2 2 0 0 0-2-2a2 2 0 0 0-2 2"/>` +
		`<path d="M14 10V9a2 2 0 0 0-2-2a2 2 0 0 0-2 2v1"/><path d="M10 9.5V4a2 2 0 0 0-2-2a2 2 0 0 0-2 2v10"/>` +
		`<path d="M18 11a2 2 0 1 1 4 0v3a8 8 0 0 1-8 8h-2c-2.8 0-4.5-.86-5.99-2.34l-3.6-3.6a2 2 0 0 1 2.83-2.82L7 15"/>`)

	// iconLaptop is a laptop, open.
	iconLaptop = mustSVG(`<rect x="4" y="4.5" width="16" height="11" rx="2" fill="currentColor" fill-opacity=".22"/><path d="M2 19.5h20"/>`)
)

// accent is one of the icons' colors, a deeper ink on the light page and a
// lighter one on the dark.
type accent struct{ light, dark core.Color }

var (
	accentCoral  = accent{core.RGB(0xe8554b), core.RGB(0xff8f86)}
	accentOrange = accent{core.RGB(0xdd7a12), core.RGB(0xffb74d)}
	accentGreen  = accent{core.RGB(0x2f8f5f), core.RGB(0x74d6a2)}
	accentBlue   = accent{core.RGB(0x2f84cc), core.RGB(0x7ccbff)}
	accentPurple = accent{core.RGB(0x7c5ce0), core.RGB(0xb9a3ff)}
)

func (a accent) of(cx *core.Scope) core.Color {
	if cx.Dark() {
		return a.dark
	}
	return a.light
}

// icon draws svg size points high in a's color.
func icon(cx *core.Scope, svg *core.SVG, a accent, size core.Points) core.Node {
	return core.Icon{SVG: svg, StyleFunc: func() core.Style {
		return core.Style{Width: core.Pt(size), Height: core.Pt(size), FlexShrink: 0, Color: a.of(cx)}
	}}
}

// badge is an icon in a rounded square of its own color's tint, as a
// setting and a tip show theirs.
func badge(cx *core.Scope, svg *core.SVG, a accent, box, size, radius core.Points) core.Node {
	return core.View{StyleFunc: func() core.Style {
		tint := float32(0.14)
		if cx.Dark() {
			tint = 0.18
		}
		return core.Style{Width: core.Pt(box), Height: core.Pt(box), CornerRadius: radius, FlexShrink: 0,
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Background: a.of(cx).Faded(tint)}
	}, Children: []core.Node{icon(cx, svg, a, size)}}
}
