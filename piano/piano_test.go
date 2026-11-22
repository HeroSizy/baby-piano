package piano

import (
	"math"
	"slices"
	"testing"
	"time"
)

func TestNotesSpanThePiano(t *testing.T) {
	if len(Notes) != 36 || Notes[0] != "C1" || Notes[5] != "C2" || Notes[35] != "C8" {
		t.Errorf("Notes = %v", Notes)
	}
	for s, n := range map[Scale]int{Pentatonic: 36, Major: 50, Chromatic: 85} {
		if p := s.Pitches(); len(p) != n || p[0] != 0 || p[len(p)-1] != Top {
			t.Errorf("%s spans %v", s, p)
		}
	}
	if Gain(0) != 1 || Gain(60) != 1 || Gain(Top) != 0.6 || Gain(80) >= Gain(79) {
		t.Errorf("gain: C1 %v, C6 %v, C8 %v", Gain(0), Gain(60), Gain(Top))
	}
	if ParseScale("major") != Major || ParseScale("") != Pentatonic || ParseScale("lydian") != Pentatonic {
		t.Error("a scale's setting misread")
	}
}

// Every pitch plays a sample at most a semitone off, a sampled one its own.
func TestEveryPitchHasASample(t *testing.T) {
	for p := 0; p <= Top; p++ {
		i, rate := Sample(p)
		semis := p - sampled[i]
		if semis < -1 || semis > 1 {
			t.Errorf("%s plays %s, %d semitones off", Name(p), Notes[i], semis)
		}
		if want := float32(math.Pow(2, float64(semis)/12)); math.Abs(float64(rate-want)) > 1e-5 {
			t.Errorf("%s plays %s at %v, want %v", Name(p), Notes[i], rate, want)
		}
	}
	if i, rate := Sample(36); Notes[i] != "C4" || rate != 1 {
		t.Errorf("C4 plays %s at %v", Notes[i], rate)
	}
	if Name(37) != "C#4" || Name(Top) != "C8" {
		t.Errorf("names: %s, %s", Name(37), Name(Top))
	}
}

func TestEveryKeyPlaysAndNeighboursDiffer(t *testing.T) {
	for _, s := range Scales {
		pitches := s.Pitches()
		for r, row := range keyRows {
			for c, name := range row {
				n := NoteForKey(s, name)
				if !slices.Contains(pitches, n) {
					t.Errorf("%s: %q = %d, off the scale", s, name, n)
				}
				if c > 0 && NoteForKey(s, row[c-1]) == n {
					t.Errorf("%s: %q and %q, side by side, both play %s", s, row[c-1], name, Name(n))
				}
			}
			if got, want := NoteForKey(s, row[0]), pitches[r*s.PerOctave()]; got != want {
				t.Errorf("%s: row %d starts at %s, want %s, an octave over the row below", s, r, Name(got), Name(want))
			}
		}
	}
	if NoteForKey(Pentatonic, "ctrl") != 0 || NoteForKey(Pentatonic, "escape") != 60 || NoteForKey(Pentatonic, "f10") != Top {
		t.Errorf("ctrl %d, escape %d, f10 %d", NoteForKey(Pentatonic, "ctrl"), NoteForKey(Pentatonic, "escape"), NoteForKey(Pentatonic, "f10"))
	}
	if NoteForKey(Pentatonic, "f11") != 81 {
		t.Errorf("f11, past the top, = %s, want A7, turning back down", Name(NoteForKey(Pentatonic, "f11")))
	}
	if NoteForKey(Chromatic, "q") != 37 || NoteForKey(Major, "q") != 38 {
		t.Errorf("q: chromatic %s, major %s", Name(NoteForKey(Chromatic, "q")), Name(NoteForKey(Major, "q")))
	}
}

func TestPlatformTablesCoverTheRows(t *testing.T) {
	named := map[string]bool{}
	for _, name := range macNames {
		named[name] = true
	}
	for _, name := range windowsNames {
		named[name+"/win"] = true
	}
	for _, row := range keyRows {
		for _, name := range row {
			if !named[name] && name != "menu" {
				t.Errorf("no Mac key is %q", name)
			}
			if !named[name+"/win"] && name != "kp+mac" {
				t.Errorf("no Windows key is %q", name)
			}
		}
	}
	for _, tc := range []struct{ mac, scan int }{{12, 0x10}, {0, 0x1E}, {50, 0x29}, {60, 0x36}, {36, 0x1C}, {53, 0x01}} {
		if m, w := MacKeyName(tc.mac), WindowsKeyName(tc.scan, false); m != w {
			t.Errorf("mac %d = %q, scan %#x = %q", tc.mac, m, tc.scan, w)
		}
	}
	if WindowsKeyName(0x1C, true) != "kpenter" || WindowsKeyName(0x45, true) != "numlock" || WindowsKeyName(0x45, false) != "pause" {
		t.Error("keypad Enter, Num Lock or Pause misnamed")
	}
	if n := NoteForKey(Pentatonic, MacMediaKeyName(0)); n < 0 || n > Top || n != NoteForKey(Pentatonic, "media:0") {
		t.Errorf("a media key = %d", n)
	}
}

func TestExitWord(t *testing.T) {
	var e ExitSequence
	now := time.Unix(0, 0)
	typeWord := func(word string, gap time.Duration) bool {
		done := false
		for _, r := range word {
			now = now.Add(gap)
			done = e.Feed(string(r), now)
		}
		return done
	}
	if !typeWord("xxQUITPIANO", 100*time.Millisecond) {
		t.Error("quitpiano after other keys did not exit")
	}
	if typeWord("quitpian", time.Second) || typeWord("o", 4*time.Second) {
		t.Error("a pause over ExitPause still exited")
	}
	if e.Feed("", now) {
		t.Error("a key typing nothing exited")
	}
}

func TestLimiter(t *testing.T) {
	var l Limiter
	now := time.Unix(0, 0)
	allowed := 0
	for range 20 { // a slap: 20 keys at once
		if l.Allow(now) {
			allowed++
		}
	}
	if allowed != Burst {
		t.Errorf("a slap played %d notes, want %d", allowed, Burst)
	}
	if !l.Allow(now.Add(Window)) {
		t.Error("no note once the window passed")
	}
	l.Mute(now.Add(10 * time.Second))
	if l.Allow(now.Add(5 * time.Second)) {
		t.Error("a note while muted")
	}
}
