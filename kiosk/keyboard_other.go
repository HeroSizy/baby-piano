//go:build !ios

package kiosk

// Keyboard reports whether a physical keyboard is attached to a phone or
// tablet. Only iOS says; elsewhere a key pressed shows it, and a computer
// always has one.
func Keyboard() bool { return false }
