package piano

import (
	"strings"
	"time"
)

// ExitWord is what a parent types to leave the piano, with no pause longer
// than ExitPause between letters. A baby pressing at random practically never
// types it.
const (
	ExitWord  = "quitpiano"
	ExitPause = 3 * time.Second
)

// ExitSequence watches typed text for ExitWord.
type ExitSequence struct {
	typed []rune
	last  time.Time
}

// Feed adds the text a key typed at now and reports whether ExitWord has
// just been completed.
func (e *ExitSequence) Feed(text string, now time.Time) bool {
	if text == "" {
		return false
	}
	if now.Sub(e.last) > ExitPause {
		e.typed = e.typed[:0]
	}
	e.last = now
	e.typed = append(e.typed, []rune(strings.ToLower(text))...)
	if n := len(e.typed) - len([]rune(ExitWord)); n > 0 {
		e.typed = append(e.typed[:0], e.typed[n:]...)
	}
	return string(e.typed) == ExitWord
}

// Limiter keeps a slap of the whole hand from being loud: at most Burst notes
// in any Window, about 12 a second, and none at all while muted.
type Limiter struct {
	recent    []time.Time
	muteUntil time.Time
}

const (
	Burst  = 5
	Window = 400 * time.Millisecond
)

// Allow reports whether a note may sound at now, and counts it if so.
func (l *Limiter) Allow(now time.Time) bool {
	if now.Before(l.muteUntil) {
		return false
	}
	kept := l.recent[:0]
	for _, t := range l.recent {
		if now.Sub(t) < Window {
			kept = append(kept, t)
		}
	}
	l.recent = kept
	if len(l.recent) >= Burst {
		return false
	}
	l.recent = append(l.recent, now)
	return true
}

// Mute silences every note until until, as after the output device changed
// to one that may be louder.
func (l *Limiter) Mute(until time.Time) { l.muteUntil = until }
