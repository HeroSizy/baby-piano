// Command baby-piano is 宝宝钢琴, a piano for babies: on a computer it takes
// the whole keyboard so every key plays a gentle note and nothing quits or
// switches apps; on a phone or tablet it is big colored keys for every
// finger, or, with a keyboard attached, the keyboard plays.
package main

import (
	"time"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"

	"github.com/HeroSizy/baby-piano/kiosk"
	"github.com/HeroSizy/baby-piano/piano"
	"github.com/HeroSizy/baby-piano/sound"
)

func main() { nsgo.Run(Root, nsgo.WindowSize(600, 620)) }

// Root is the app on this platform's kiosk.
func Root(cx *core.Scope) core.Node { return newRoot(kiosk.System{})(cx) }

// locker is what the app asks of the platform's kiosk (kiosk.System), so
// tests can stand in for it without taking the computer's keyboard.
type locker interface {
	Supported() bool
	SecureInput() bool
	Start(kiosk.Options, func(kiosk.Key)) error
	Stop()
	Enter()
	Front()
	Tick() (outputChanged bool)
	SystemVolume() (float32, bool)
}

// settings are the parents' choices, kept between launches.
type settings struct {
	volume    *core.Persisted[float64]
	lockMouse *core.Persisted[bool]
	visuals   *core.Persisted[bool]
	scale     *core.Persisted[piano.Scale]
}

func loadSettings(cx *core.Scope) settings {
	return settings{
		volume:    core.NewPersisted(cx, "volume", 0.45, core.PersistOptions{}),
		lockMouse: core.NewPersisted(cx, "lockMouse", false, core.PersistOptions{}),
		visuals:   core.NewPersisted(cx, "showVisuals", false, core.PersistOptions{}),
		scale:     core.NewPersisted(cx, "scale", piano.Pentatonic, core.PersistOptions{}),
	}
}

// app is what the screens share: the settings, the loaded piano and
// whether the baby is playing.
type app struct {
	cx       *core.Scope
	kiosk    locker
	settings settings
	piano    *core.Signal[*sound.Piano]
	soundErr *core.Signal[error]
	playing  *core.Signal[bool]
	// locked is whether kiosk took the keyboard: a computer's way to play.
	locked bool
	// session counts the times playing started, so a lock's tick stops
	// once that session ends, even when the next one starts at once.
	session int
	// burst draws a note's colored circle, when the play screen shows them.
	burst func(note int)
	// tapAt plays the touch key at a point as a finger landing there does,
	// while a phone or tablet shows them; the store screenshots tap with it.
	tapAt func(at core.Point)
	exit  piano.ExitSequence
	stop  []func()
}

// newRoot is the app on lock: the parents' setup, then the piano.
func newRoot(lock locker) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node { return newApp(cx, lock).root() }
}

func newApp(cx *core.Scope, lock locker) *app {
	return &app{
		kiosk:    lock,
		cx:       cx,
		settings: loadSettings(cx),
		piano:    core.NewSignal[*sound.Piano](cx, nil),
		soundErr: core.NewSignal[error](cx, nil),
		playing:  core.NewSignal(cx, false),
	}
}

func (a *app) root() core.Node {
	cx := a.cx
	sound.Load(cx, float32(a.settings.volume.Get()), func(p *sound.Piano, err error) {
		a.piano.Set(p)
		a.soundErr.Set(err)
	})
	cx.Cleanup(func() {
		if p := a.piano.GetUntracked(); p != nil {
			p.Close()
		}
	})
	if desktop.IsAvailable(cx) {
		// While the baby plays, only the parents' exit quits: should a
		// Cmd+Q ever get past the lock, it does nothing.
		desktop.OnQuitRequest(cx, func(q *core.QuitRequest) {
			if a.playing.GetUntracked() && !q.Forced() {
				q.Cancel()
			}
		})
	}
	return core.Show{
		When: a.playing.Get,
		Then: a.playScreen,
		Else: a.setupScreen,
	}
}

// scale is the parents' scale, read without tracking it.
func (a *app) scale() piano.Scale { return core.Untracked(a.cx, a.settings.scale.Get) }

// play sounds note i of the parents' scale and reports whether it sounded.
func (a *app) play(i int) bool {
	p := a.piano.GetUntracked()
	return p != nil && p.Play(a.scale().Pitch(i))
}

// start takes over the computer where it can and shows the piano. It
// returns why it could not lock the keyboard, if it tried and failed.
func (a *app) start() error {
	cx := a.cx
	if a.kiosk.Supported() {
		if a.kiosk.SecureInput() {
			return errSecureInput
		}
		opts := kiosk.Options{LockMouse: a.settings.lockMouse.Get()}
		if err := a.kiosk.Start(opts, func(k kiosk.Key) { cx.Post(func() { a.key(k.Name, k.Text) }) }); err != nil {
			return err
		}
		a.locked = true
		desktop.SetApplicationMenu(cx, nil) // no menu, no Cmd+Q
		if cx.Env().OS() == core.OSWindows && !desktop.FullScreen(cx) {
			desktop.ToggleFullScreen(cx) // the kiosk keeps it on top; macOS's covers the screens itself
		}
		a.kiosk.Enter()
		a.stop = append(a.stop, cx.KeepAwake("A baby is playing the piano", core.KeepAwakeOptions{Display: true}))
		a.session++
		a.tick(a.session)
	} else if !desktop.IsAvailable(cx) {
		a.stop = append(a.stop, cx.KeepAwake("A baby is playing the piano", core.KeepAwakeOptions{Display: true}))
	}
	a.playing.Set(true)
	return nil
}

// tick keeps the computer locked every half second while session plays: the lock
// on, the volume capped, a moment's silence after the output changed, and
// the piano in front.
func (a *app) tick(session int) {
	if !a.locked || session != a.session {
		return
	}
	if a.kiosk.Tick() {
		if p := a.piano.GetUntracked(); p != nil {
			p.Pause(time.Second)
		}
	}
	a.kiosk.Front()
	a.cx.AfterFunc(500*time.Millisecond, func() { a.tick(session) })
}

// key plays the key named name, which typed text, unless it completes the
// parents' exit word.
func (a *app) key(name, text string) {
	if a.exit.Feed(text, a.cx.Now()) {
		a.leave()
		return
	}
	i := piano.NoteForKey(name)
	if a.play(i) && a.burst != nil {
		a.burst(i)
	}
}

// leave ends playing and goes back to the setup: a computer first gives the
// keyboard back, uncovers the screens and leaves full screen.
func (a *app) leave() {
	for _, stop := range a.stop {
		stop()
	}
	a.stop = nil
	if a.locked {
		a.locked = false
		a.kiosk.Stop()
		desktop.SetApplicationMenu(a.cx, nil) // the default menu, Cmd+Q again
		if a.cx.Env().OS() == core.OSWindows && desktop.FullScreen(a.cx) {
			desktop.ToggleFullScreen(a.cx)
		}
	}
	a.playing.Set(false)
}
