//go:build (!darwin && !windows) || ios

package kiosk

import "errors"

// Supported reports whether this platform locks the keyboard: phones,
// tablets and the rest leave locking to the system.
func Supported() bool { return false }

// SecureInput reports whether another app turned on secure keyboard entry.
func SecureInput() bool { return false }

// ErrNotAllowed is Start's error where the keyboard cannot be locked.
var ErrNotAllowed = errors.New("kiosk: this platform does not lock the keyboard")

// Start fails: there is no keyboard to lock here.
func Start(Options, func(Key)) error { return ErrNotAllowed }

// Stop does nothing.
func Stop() {}

// Tick does nothing.
func Tick() (outputChanged bool) { return false }

// Enter does nothing; the system's Guided Access or screen pinning locks the
// device to the app.
func Enter() {}

// Front does nothing.
func Front() {}

// SystemVolume is unknown here.
func SystemVolume() (float32, bool) { return 0, false }
