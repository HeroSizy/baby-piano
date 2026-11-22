package main

import "testing"

func TestPianoKeyAt(t *testing.T) {
	for _, c := range []struct {
		fx, fy float64
		want   int
	}{
		{0.05, 0.9, 0}, {0.05, 0.2, 0}, {1.0 / 7, 0.2, 1}, {1.0 / 7, 0.8, 2}, {3.0 / 7, 0.3, 5},
		{4.0 / 7, 0.3, 6}, {6.0 / 7, 0.3, 10}, {0.99, 0.3, 11},
	} {
		if got := pianoKeyAt(c.fx, c.fy); got != c.want {
			t.Errorf("pianoKeyAt(%.2f, %.2f) = %d, want %d", c.fx, c.fy, got, c.want)
		}
	}
}
