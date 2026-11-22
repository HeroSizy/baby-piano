package main

import (
	"os"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/HeroSizy/baby-piano/kiosk"
)

// TestStoreScreenshots poses the App Store screenshots on a phone or tablet
// simulator, in dark mode and English: the setup page, then the keys, then
// the keys with fireworks, each held for a few seconds on the real clock.
// The app's own capture leaves out animations in flight and the status
// bar, so tools/store-screenshots.sh runs it and takes the pictures from
// outside with `xcrun simctl io <udid> screenshot`. It runs only when asked,
// with STORE_SCREENSHOTS set.
func TestStoreScreenshots(t *testing.T) {
	if os.Getenv("STORE_SCREENSHOTS") == "" {
		t.Skip("set STORE_SCREENSHOTS=1 to pose the store screenshots")
	}
	var a *app
	root := func(cx *core.Scope) core.Node {
		a = newApp(cx, kiosk.System{})
		return a.root()
	}
	app := nsgotest.Start(t, root, nsgotest.Measure(),
		nsgotest.Environment(core.Environment{Audio: true, Dark: true, Locale: "en"}))
	if a.kiosk.Supported() {
		t.Skip("store screenshots are of the touch keys")
	}
	hold := func(d time.Duration) {
		app.Settle()
		app.Advance(d)
	}

	app.WaitUntil(10*time.Second, func() bool { return app.Enabled("id:start") })
	app.Post(func() { _ = a.settings.visuals.Set(true) })
	hold(4 * time.Second)

	app.Press("id:start")
	size := app.WaitFor("id:keys", 5*time.Second).Frame.Size
	hold(4 * time.Second)

	// Fingers land on the keys every 160 ms, so several fireworks are in
	// the air at once. The spots are fractions of the screen, spread over
	// it.
	spots := [][2]float64{
		{0.30, 0.20}, {0.70, 0.62}, {0.18, 0.47}, {0.82, 0.30},
		{0.45, 0.80}, {0.60, 0.42}, {0.22, 0.70}, {0.75, 0.86},
		{0.40, 0.33}, {0.86, 0.55}, {0.12, 0.25}, {0.55, 0.62},
	}
	n := 0
	var tap func()
	tap = func() {
		if a.tapAt != nil {
			s := spots[n%len(spots)]
			a.tapAt(core.Point{X: core.Points(s[0]) * size.Width, Y: core.Points(s[1]) * size.Height})
		}
		n++
		a.cx.AfterFunc(160*time.Millisecond, tap)
	}
	app.Post(tap)
	app.Settle() // runs the post; never at rest, it returns after a few seconds
	app.Advance(10 * time.Second)
}
