// Package sound plays the piano's notes through NSGo's audio package.
package sound

import (
	"embed"
	"fmt"
	"time"

	"github.com/nsgo-dev/nsgo/audio"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/HeroSizy/baby-piano/piano"
)

//go:embed notes/*.wav
var notes embed.FS

// headroom scales every voice so that several notes ringing together stay
// below full scale: the samples peak at −3 dBFS, and the limiter lets up to
// piano.Burst start within a moment.
const headroom = 0.5

// Piano is the loaded notes, played with the parent's volume and the
// limiter's pace. Use it on the engine's goroutine, as every NSGo service.
type Piano struct {
	sounds  []*audio.Sound
	limiter piano.Limiter
	volume  float32
	now     func() time.Time
}

// Load decodes every note of piano.Notes and calls done with the piano, or
// with the first error; a platform that plays no audio fails with
// audio.Unavailable.
func Load(cx *core.Scope, volume float32, done func(*Piano, error)) {
	p := &Piano{sounds: make([]*audio.Sound, len(piano.Notes)), volume: volume, now: cx.Now}
	pending, failed := len(piano.Notes), false
	for i, data := range Samples() {
		name := piano.Notes[i]
		audio.LoadSound(cx, audio.Source{Data: data}, func(s *audio.Sound, err error) {
			if failed {
				return
			}
			if err != nil {
				failed = true
				p.Close()
				done(nil, fmt.Errorf("note %s: %w", name, err))
				return
			}
			p.sounds[i] = s
			if pending--; pending == 0 {
				done(p, nil)
			}
		})
	}
}

// Play sounds note index i of piano.Notes, clamped to the scale, and reports
// whether it sounded: false while the limiter holds notes back.
func (p *Piano) Play(i int) bool {
	if !p.limiter.Allow(p.now()) {
		return false
	}
	i = min(max(i, 0), len(p.sounds)-1)
	p.sounds[i].Play(audio.Voice{Volume: p.volume * headroom * piano.Gain(i)})
	return true
}

// SetVolume sets the parent's volume, from 0.05 to 0.9, for notes played from now on.
func (p *Piano) SetVolume(v float32) { p.volume = min(max(v, 0.05), 0.9) }

// Pause holds every note back for d, as after the output device changed to
// one that may be louder.
func (p *Piano) Pause(d time.Duration) { p.limiter.Mute(p.now().Add(d)) }

// Close frees the notes.
func (p *Piano) Close() {
	for _, s := range p.sounds {
		if s != nil {
			s.Close()
		}
	}
}

// Samples are the notes' WAV files, in piano.Notes' order.
func Samples() [][]byte {
	samples := make([][]byte, len(piano.Notes))
	for i, name := range piano.Notes {
		data, err := notes.ReadFile("notes/" + name + ".wav")
		if err != nil {
			panic(err) // embedded at build time
		}
		samples[i] = data
	}
	return samples
}
