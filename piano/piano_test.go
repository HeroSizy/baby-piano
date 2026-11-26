package piano

import (
	"testing"
	"time"
)

func TestNotesSpanThePiano(t *testing.T) {
	if len(Notes) != 36 || Notes[0] != "C1" || Notes[5] != "C2" || Notes[35] != "C8" {
		t.Errorf("Notes = %v", Notes)
	}
	g := func(i int) float32 { return PitchGain(Pentatonic.Pitch(i)) }
	if g(0) != 1 || g(25) != 1 || g(35) != 0.6 || g(30) >= g(29) {
		t.Errorf("gain: C1 %v, C6 %v, C8 %v", g(0), g(25), g(35))
	}
}

func TestScales(t *testing.T) {
	for _, s := range Scales {
		if got := NoteName(s.Pitch(s.From())); got != "C2" {
			t.Errorf("%s: note %d is %s, want C2", s, s.From(), got)
		}
		if got := NoteName(s.Pitch(s.From() + s.PerOctave())); got != "C3" {
			t.Errorf("%s: an octave up is %s, want C3", s, got)
		}
		for i := range Notes {
			p := s.Pitch(i)
			if i > 0 && p <= s.Pitch(i-1) {
				t.Errorf("%s: note %d (%s) is not above the one below", s, i, NoteName(p))
			}
			j, semis := Sample(p)
			if semis < -1 || semis > 1 {
				t.Errorf("%s: %s is %d semitones from sample %s", s, NoteName(p), semis, Notes[j])
			}
			if s == Pentatonic && (j != i || semis != 0) {
				t.Errorf("pentatonic note %d plays sample %d shifted %d", i, j, semis)
			}
		}
	}
	for i, want := range map[int]string{0: "G1", 13: "F3", 16: "B3", 35: "G6"} {
		if got := NoteName(Major.Pitch(i)); got != want {
			t.Errorf("major note %d = %s, want %s", i, got, want)
		}
	}
	if NoteName(Chromatic.Pitch(0)) != "C2" || NoteName(Chromatic.Pitch(13)) != "C#3" || NoteName(Chromatic.Pitch(35)) != "B4" {
		t.Errorf("chromatic: %s %s %s", NoteName(Chromatic.Pitch(0)), NoteName(Chromatic.Pitch(11)), NoteName(Chromatic.Pitch(35)))
	}
}

func TestEveryKeyPlaysAndNeighboursDiffer(t *testing.T) {
	for r, row := range keyRows {
		for c, name := range row {
			n := NoteForKey(name)
			if n < 0 || n >= len(Notes) {
				t.Errorf("%q = %d, off the scale", name, n)
			}
			if c > 0 && NoteForKey(row[c-1]) == n {
				t.Errorf("%q and %q, side by side, both play %s", row[c-1], name, Notes[n])
			}
		}
		if got, want := NoteForKey(row[0]), r*rowStep; got != want {
			t.Errorf("row %d starts at %d, want %d, an octave over the row below", r, got, want)
		}
	}
	if NoteForKey("ctrl") != 0 || NoteForKey("escape") != 25 || NoteForKey("f10") != 35 {
		t.Errorf("ctrl %d, escape %d, f10 %d", NoteForKey("ctrl"), NoteForKey("escape"), NoteForKey("f10"))
	}
	if NoteForKey("f11") != 34 {
		t.Errorf("f11, past the top, = %d, want 34, turning back down", NoteForKey("f11"))
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
	if n := NoteForKey(MacMediaKeyName(0)); n < 0 || n >= len(Notes) || n != NoteForKey("media:0") {
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

func TestLetter(t *testing.T) {
	for p, want := range map[int]string{60: "C", 61: "C#", 69: "A", 70: "A#", 23: "B", 36: "C"} {
		if got := Letter(p); got != want {
			t.Errorf("Letter(%d) = %q, want %q", p, got, want)
		}
	}
}
