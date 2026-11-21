package main

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/HeroSizy/baby-piano/kiosk"
	"github.com/HeroSizy/baby-piano/piano"
	"github.com/HeroSizy/baby-piano/sound"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

// onMac runs the app as on a Mac, where it locks the keyboard.
var onMac = nsgotest.Environment(core.Environment{Audio: true, OS: core.OSMacOS, Idiom: core.IdiomDesktop})

// fakeKiosk stands in for the computer's kiosk, which would take the
// keyboard of whoever runs the tests.
type fakeKiosk struct {
	supported, started, entered, stopped bool
	opts                                 kiosk.Options
	onKey                                func(kiosk.Key)
	volume                               float32
}

func (f *fakeKiosk) Supported() bool   { return f.supported }
func (f *fakeKiosk) SecureInput() bool { return false }
func (f *fakeKiosk) Start(opts kiosk.Options, onKey func(kiosk.Key)) error {
	f.started, f.opts, f.onKey = true, opts, onKey
	return nil
}
func (f *fakeKiosk) Stop()                         { f.stopped = true }
func (f *fakeKiosk) Enter()                        { f.entered = true }
func (f *fakeKiosk) Front()                        {}
func (f *fakeKiosk) Tick() bool                    { return false }
func (f *fakeKiosk) SystemVolume() (float32, bool) { return f.volume, true }

// start runs the app on k with every note decodable, a second long, as the
// platform would decode the embedded samples.
func start(t *testing.T, k *fakeKiosk, opts ...nsgotest.Option) *nsgotest.App {
	durations := map[string]time.Duration{}
	for _, data := range sound.Samples() {
		durations[string(data)] = time.Second
	}
	opts = append([]nsgotest.Option{nsgotest.Environment(core.Environment{Audio: true})}, opts...)
	opts = append(opts, func(a *nsgotest.App) { a.AudioDurations = durations })
	app := nsgotest.Start(t, newRoot(k), opts...)
	if len(app.Sounds) != len(piano.Notes) {
		t.Fatalf("%d notes loaded, want %d", len(app.Sounds), len(piano.Notes))
	}
	return app
}

// plays counts each note's playbacks, by index in piano.Notes.
func plays(app *nsgotest.App) []int {
	n := make([]int, len(app.Sounds))
	for i, s := range app.Sounds {
		n[i] = len(s.Plays)
	}
	return n
}

func total(n []int) (sum int) {
	for _, v := range n {
		sum += v
	}
	return sum
}

func TestTryItPlaysAChord(t *testing.T) {
	app := start(t, &fakeKiosk{supported: true}, onMac)
	app.Press("id:try")
	app.Advance(time.Second)
	if got := total(plays(app)); got != 4 {
		t.Errorf("Try it played %d notes, want 4", got)
	}
}

func TestComputerLocksAndKeysPlay(t *testing.T) {
	k := &fakeKiosk{supported: true, volume: 0.3}
	app := start(t, k, onMac)
	if app.Enabled("id:start") {
		t.Fatal("start is enabled without the Accessibility permission")
	}
	app.GrantPermission(core.PermissionAccessibility)
	app.Advance(time.Second) // the setup reads the permission again
	app.Press("id:start")
	if !k.started || !k.entered {
		t.Fatalf("start: lock %v, screens covered %v", k.started, k.entered)
	}
	if !app.Exists("id:corner") {
		t.Fatal("no parents' corner on the piano")
	}

	press := func(name, text string) {
		k.onKey(kiosk.Key{Name: name, Text: text})
		app.Settle()
	}
	press("a", "a")
	if n := plays(app); n[piano.NoteForKey("a")] != 1 || total(n) != 1 {
		t.Errorf("a key played %v", n)
	}

	// A slap of the whole hand plays a few notes, not twenty.
	app.Advance(time.Second)
	before := total(plays(app))
	for _, name := range []string{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "z", "x", "c", "v", "b", "n", "m", ",", ".", "/"} {
		k.onKey(kiosk.Key{Name: name})
	}
	app.Settle()
	if got := total(plays(app)) - before; got != piano.Burst {
		t.Errorf("a slap played %d notes, want %d", got, piano.Burst)
	}

	for _, r := range "quitpiano" {
		press(piano.MacKeyName(0), string(r))
	}
	if !k.stopped || app.Quits != 1 {
		t.Errorf("quitpiano: keyboard given back %v, quits %d", k.stopped, app.Quits)
	}
}

func TestQuitIsRefusedWhilePlaying(t *testing.T) {
	k := &fakeKiosk{supported: true}
	app := start(t, k, onMac)
	app.GrantPermission(core.PermissionAccessibility)
	app.Advance(time.Second)
	app.Press("id:start")
	app.RequestQuit(false)
	if app.Quits != 0 {
		t.Error("Cmd+Q quit the playing piano")
	}
}

func TestTouchKeysPlayEveryFinger(t *testing.T) {
	app := start(t, &fakeKiosk{}, nsgotest.Size(400, 800))
	app.Press("id:start")
	if !app.Exists("id:corner") {
		t.Fatal("no piano")
	}
	// Four columns of four rows upright: the second key of the third row
	// and the last key.
	b := app.Find("id:keys").Bounds()
	w, h := b.Size.Width/4, b.Size.Height/4
	f1 := app.Touch("id:keys", core.Point{X: w * 1.5, Y: h * 2.5})
	f2 := app.Touch("id:keys", core.Point{X: w * 3.5, Y: h * 3.5})
	f1.Lift()
	f2.Lift()
	n := plays(app)
	if n[touchFirst+9] != 1 || n[touchFirst+15] != 1 || total(n) != 2 {
		t.Errorf("two fingers played %v", n)
	}
}

func TestHoldingTheCornerStops(t *testing.T) {
	// Manual frames, so a step does not wait for the ring to fill.
	app := start(t, &fakeKiosk{}, nsgotest.Size(400, 800), nsgotest.ManualFrames())
	app.Press("id:start")
	finger := app.Touch("id:corner", core.Point{X: 45, Y: 45})
	app.Advance(time.Second)
	finger.Lift()
	if !app.Exists("id:corner") {
		t.Fatal("a short hold stopped the piano")
	}
	finger = app.Touch("id:corner", core.Point{X: 45, Y: 45})
	app.Advance(exitHold + 100*time.Millisecond)
	finger.Lift()
	if app.Exists("id:corner") || !app.Exists("id:start") {
		t.Error("holding the corner did not go back to the setup")
	}
}

// TestScreens saves the screens as they show on a platform (go tool nsgo
// test -target macos -run TestScreens); headless it checks nothing more.
func TestScreens(t *testing.T) {
	k := &fakeKiosk{supported: true, volume: 0.7}
	app := nsgotest.Start(t, newRoot(k), nsgotest.Size(600, 620))
	app.WaitUntil(5*time.Second, func() bool { return !app.Exists("text:" + "Loading the piano…") })
	app.Screenshot("setup-computer")
	t.Logf("status: %q", app.Find("id:status").Text)
}

// TestFireworksKeepUpWithMashing plays more notes than the effects' pools
// hold, with and without reduced motion: every note still sounds as the
// oldest firework is reused.
func TestFireworksKeepUpWithMashing(t *testing.T) {
	for _, still := range []bool{false, true} {
		k := &fakeKiosk{supported: true}
		app := start(t, k, nsgotest.Environment(core.Environment{Audio: true, OS: core.OSMacOS, Idiom: core.IdiomDesktop, ReduceMotion: still}))
		app.GrantPermission(core.PermissionAccessibility)
		app.Advance(time.Second)
		app.Press("id:visuals")
		app.Press("id:start")
		for i := range 3 * burstSlots {
			k.onKey(kiosk.Key{Name: string(rune('a' + i%26))})
			app.Advance(100 * time.Millisecond)
		}
		if got := total(plays(app)); got != 3*burstSlots {
			t.Errorf("reduced motion %v: %d notes sounded, want %d", still, got, 3*burstSlots)
		}
	}
}

// TestLockedMouseHidesThePointer hides the pointer over the piano only
// while the mouse is locked: unlocked, parents need it for the corner.
func TestLockedMouseHidesThePointer(t *testing.T) {
	for _, locked := range []bool{false, true} {
		app := start(t, &fakeKiosk{supported: true}, onMac)
		app.GrantPermission(core.PermissionAccessibility)
		app.Advance(time.Second)
		if locked {
			app.Press("id:lockMouse")
		}
		app.Press("id:start")
		if hidden := app.Find("id:piano").Style.Cursor == core.CursorNone; hidden != locked {
			t.Errorf("mouse locked %v: pointer hidden %v", locked, hidden)
		}
	}
}
