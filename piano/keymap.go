package piano

import "hash/fnv"

// keyRows are a full-size keyboard's six rows, from the bottom up, each from
// left to right across the main block, the navigation keys and the keypad,
// by the names the platform tables give their keys (MacKeyName,
// WindowsKeyName). A few names are one platform's own: "kp+mac" is a Mac
// keypad's + one row lower than a PC's.
var keyRows = [][]string{
	{"ctrl", "meta", "alt", "space", "ralt", "rmeta", "menu", "rctrl", "left", "down", "right", "kp0", "kp."},
	{"shift", "z", "x", "c", "v", "b", "n", "m", ",", ".", "/", "rshift", "up", "kp1", "kp2", "kp3", "kpenter"},
	{"capslock", "a", "s", "d", "f", "g", "h", "j", "k", "l", ";", "'", "enter", "kp4", "kp5", "kp6", "kp+mac"},
	{"tab", "q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "[", "]", "\\", "delete", "end", "pagedown", "kp7", "kp8", "kp9", "kp+"},
	{"`", "1", "2", "3", "4", "5", "6", "7", "8", "9", "0", "-", "=", "backspace", "insert", "home", "pageup", "numlock", "kp/", "kp*", "kp-"},
	{"escape", "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12", "print", "scrolllock", "pause"},
}

// keyNotes places each key of keyRows on scale s, by index in its
// Pitches. A row starts an octave above the one below it and climbs one
// note a key to the right; past the top note it turns back down, so
// neighbouring keys always differ and every key of the keyboard plays.
func keyNotes(s Scale) map[string]int {
	notes := map[string]int{}
	top := len(s.Pitches()) - 1
	for r, row := range keyRows {
		for c, name := range row {
			i := (r*s.PerOctave() + c) % (2 * top)
			if i > top {
				i = 2*top - i
			}
			notes[name] = i
		}
	}
	return notes
}

// keyMaps are keyNotes of each scale.
var keyMaps = func() map[Scale]map[string]int {
	maps := map[Scale]map[string]int{}
	for _, s := range Scales {
		maps[s] = keyNotes(s)
	}
	return maps
}()

// NoteForKey returns the pitch the key named name plays in scale s. The same
// key always plays the same note; a key off the rows, such as a media key,
// plays a fixed note spread by its name.
func NoteForKey(s Scale, name string) int {
	s = ParseScale(string(s))
	pitches := s.Pitches()
	if i, ok := keyMaps[s][name]; ok {
		return pitches[i]
	}
	h := fnv.New32a()
	h.Write([]byte(name))
	return pitches[h.Sum32()%uint32(len(pitches))]
}
