package piano

import (
	"strconv"
	"strings"
)

// Scale is which notes the keys play, chosen by the parents. Every scale
// has as many notes as Notes, low to high, so a computer's keys and the
// fireworks lay out the same whichever it is.
type Scale string

const (
	// Pentatonic is Notes: do re mi sol la, C1 to C8. Any of them sound
	// well together, however a baby presses.
	Pentatonic Scale = "pentatonic"
	// Major is the white keys, do re mi fa sol la ti, G1 to G6.
	Major Scale = "major"
	// Chromatic is every key, white and black, C2 to B4.
	Chromatic Scale = "chromatic"
)

// Scales are the scales in the order the parents see them.
var Scales = []Scale{Pentatonic, Major, Chromatic}

// c3 is C3's MIDI pitch, the note at index middle in Notes and Major.
const (
	c3     = 48
	middle = 10
)

// Pitch is the MIDI pitch of note index i, clamped to the scale.
func (s Scale) Pitch(i int) int {
	i = min(max(i, 0), len(Notes)-1)
	switch s {
	case Major:
		steps := [7]int{0, 2, 4, 5, 7, 9, 11}
		n := i - middle + 7*10 // degrees above C3, kept positive
		return c3 + 12*(n/7-10) + steps[n%7]
	case Chromatic:
		return c3 - 12 + i
	}
	return samplePitches[i]
}

// samplePitches are Notes' MIDI pitches.
var samplePitches = func() []int {
	p := make([]int, len(Notes))
	for i, n := range Notes {
		p[i] = midi(n)
	}
	return p
}()

// midi is the MIDI pitch of a note named as Notes names them, such as C4.
func midi(name string) int {
	step := map[byte]int{'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}[name[0]]
	octave, _ := strconv.Atoi(name[1:])
	return 12*(octave+1) + step
}

// Sample is the sample nearest MIDI pitch p, by index in Notes, and how
// many semitones p is above it: at most one either way, since the samples
// are never more than three semitones apart, so a note played from a
// neighbor's sample sounds as natural as a sampler's.
func Sample(p int) (i, semitones int) {
	best := 0
	for j, q := range samplePitches {
		if abs(q-p) < abs(samplePitches[best]-p) {
			best = j
		}
	}
	return best, p - samplePitches[best]
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// NoteName names MIDI pitch p, such as C4 or F#3.
func NoteName(p int) string {
	names := [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	return names[p%12] + strconv.Itoa(p/12-1)
}

// Letter is MIDI pitch p's note name without its octave, as a key shows
// it, such as C or F#.
func Letter(p int) string { return strings.TrimRight(NoteName(p), "-0123456789") }

// PitchGain is how loud MIDI pitch p plays relative to the parent's volume:
// from C6 up the notes fade, to 60% at C8, so the highest are not piercing.
func PitchGain(p int) float32 {
	const from, top = 84, 108 // C6, C8
	if p <= from {
		return 1
	}
	return 1 - 0.4*float32(min(p, top)-from)/float32(top-from)
}

// PerOctave is how many notes of the scale make an octave.
func (s Scale) PerOctave() int {
	switch s {
	case Major:
		return 7
	case Chromatic:
		return 12
	}
	return 5
}

// From is the index of the scale's C2, where a phone or tablet's keys start,
// an octave to a row.
func (s Scale) From() int {
	switch s {
	case Major:
		return 3
	case Chromatic:
		return 0
	}
	return 5
}
