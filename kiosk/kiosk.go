// Package kiosk takes over the computer while the baby plays: every key goes
// to the piano and none to the system, the piano's window covers the
// screens, and the computer stays awake with its volume capped. macOS and
// Windows can; phones and tablets cannot, and leave locking to the system's
// own Guided Access or screen pinning (Supported is false there).
//
// Functions other than Start's key callback are called on the engine's
// goroutine; Start calls back on a goroutine of its own.
package kiosk

// Key is a key pressed while the keyboard is locked: its name for
// piano.NoteForKey, and the text it types, for the parents' exit word.
type Key struct {
	Name, Text string
}

// Options say what to lock beyond the keyboard.
type Options struct {
	// LockMouse also swallows the mouse and trackpad, so a laptop's
	// trackpad plays nothing and opens nothing; parents then leave with
	// the exit word alone.
	LockMouse bool
}

// MaxSystemVolume is the highest the system's output volume may be while the
// baby plays, from 0 to 1.
const MaxSystemVolume = 0.5

// System is this platform's kiosk, as a value apps can stand in for in
// tests: each method calls the package's function of its name.
type System struct{}

func (System) Supported() bool                           { return Supported() }
func (System) SecureInput() bool                         { return SecureInput() }
func (System) Start(opts Options, onKey func(Key)) error { return Start(opts, onKey) }
func (System) Stop()                                     { Stop() }
func (System) Enter()                                    { Enter() }
func (System) Front()                                    { Front() }
func (System) Tick() bool                                { return Tick() }
func (System) SystemVolume() (float32, bool)             { return SystemVolume() }
func (System) Keyboard() bool                            { return Keyboard() }

// Output is a sound output the computer can play through: its speakers,
// headphones, a USB or Bluetooth speaker, a display's sound.
type Output struct {
	// ID names the output to SetOutput.
	ID string
	// Name is what the system calls it, in the system's language.
	Name string
	// Default is whether the system plays through it now.
	Default bool
}

func (System) Outputs() []Output        { return Outputs() }
func (System) SetOutput(id string) bool { return SetOutput(id) }
