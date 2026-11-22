// Package piano holds what the baby piano decides, apart from any platform:
// which note a key plays, how loud it is, how fast notes may come, and the
// parents' exit word.
package piano

import "strconv"

// A note is a pitch: semitones above C1, so 0 is C1, 36 middle C (C4)
// and Top C8, the top of a piano.
const Top = 84

// Notes are the sampled notes, the C major pentatonic scale from C1 to C8.
// Each names its sample, sound/notes/<name>.wav; every other note plays the
// sample a semitone from it, pitched (Sample).
var Notes = func() []string {
	var notes []string
	for _, p := range sampled {
		notes = append(notes, Name(p))
	}
	return notes
}()

// sampled are Notes' pitches.
var sampled = Pentatonic.Pitches()

// Scale is the notes the piano plays, from C1 to C8.
type Scale string

const (
	// Pentatonic is C D E G A: any of them sound well together, however a
	// baby presses. The default.
	Pentatonic Scale = "pentatonic"
	// Major is the C major scale, the white keys: do re mi fa sol la ti.
	Major Scale = "major"
	// Chromatic is all twelve notes of each octave, black keys and white.
	Chromatic Scale = "chromatic"
)

// Scales are the scales parents choose from, the default first.
var Scales = []Scale{Pentatonic, Major, Chromatic}

// steps are each scale's notes within an octave, in semitones above C.
var steps = map[Scale][]int{
	Pentatonic: {0, 2, 4, 7, 9},
	Major:      {0, 2, 4, 5, 7, 9, 11},
	Chromatic:  {0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
}

// ParseScale is the scale named s, or Pentatonic for any other name, as a
// setting from an older or newer version.
func ParseScale(s string) Scale {
	if _, ok := steps[Scale(s)]; ok {
		return Scale(s)
	}
	return Pentatonic
}

// PerOctave is how many of the scale's notes an octave holds.
func (s Scale) PerOctave() int { return len(steps[ParseScale(string(s))]) }

// Pitches are the scale's notes from C1 to C8, low to high.
func (s Scale) Pitches() []int {
	var pitches []int
	for octave := 0; octave*12 < Top; octave++ {
		for _, step := range steps[ParseScale(string(s))] {
			pitches = append(pitches, octave*12+step)
		}
	}
	return append(pitches, Top)
}

// Between are the scale's notes from lo to hi, both included.
func (s Scale) Between(lo, hi int) []int {
	var pitches []int
	for _, p := range s.Pitches() {
		if p >= lo && p <= hi {
			pitches = append(pitches, p)
		}
	}
	return pitches
}

// Name is pitch p's name, as "C4" or "F#4".
func Name(p int) string {
	return []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}[p%12] + strconv.Itoa(p/12+1)
}

// Sample returns the index in Notes of the sample pitch p plays and the
// rate that pitches it there: the nearest sampled note is never more than
// a semitone away, as a sampler would stretch it.
func Sample(p int) (i int, rate float32) {
	p = min(max(p, 0), Top)
	best := 0
	for j, q := range sampled {
		if abs(q-p) < abs(sampled[best]-p) {
			best = j
		}
	}
	semis := p - sampled[best]
	return best, [...]float32{0.943874, 1, 1.059463}[semis+1] // 2^(semis/12)
}

func abs(n int) int { return max(n, -n) }

// Gain is how loud pitch p plays relative to the parent's volume: the top
// two octaves fade to 60% so the highest notes are not piercing.
func Gain(p int) float32 {
	const from = 60 // C6
	if p <= from {
		return 1
	}
	return 1 - 0.4*float32(min(p, Top)-from)/float32(Top-from)
}
