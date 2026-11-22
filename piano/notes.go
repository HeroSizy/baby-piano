// Package piano holds what the baby piano decides, apart from any platform:
// which note a key plays, how loud it is, how fast notes may come, and the
// parents' exit word.
package piano

// Notes are the C major pentatonic scale over a piano's range, C1 to C8, low
// to high: any of them sound well together, however a baby presses. Each
// names its sample, sound/notes/<name>.wav.
var Notes = func() []string {
	var notes []string
	for _, octave := range "1234567" {
		for _, step := range "CDEGA" {
			notes = append(notes, string(step)+string(octave))
		}
	}
	return append(notes, "C8")
}()
