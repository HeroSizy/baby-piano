package main

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/HeroSizy/baby-piano/locales"
)

// TestEveryLanguageShowsItsSetup opens the setup, on a Mac and on a phone,
// in each language the app speaks, and in a regional locale that falls to
// one of them: the start button says that language's words.
func TestEveryLanguageShowsItsSetup(t *testing.T) {
	locs := append(locales.Catalog.Languages(), "zh-TW", "zh-HK", "pt-PT", "es-MX", "fr-CA", "de-AT", "nb-NO", "no")
	for _, loc := range locs {
		msgs := locales.In(loc)
		if loc != "en" && msgs.Start() == locales.In("en").Start() {
			t.Errorf("%s: falls to English", loc)
		}
		for _, env := range []core.Environment{
			{Audio: true, Locale: loc, OS: core.OSMacOS, Idiom: core.IdiomDesktop},
			{Audio: true, Locale: loc, OS: core.OSIOS, Idiom: core.IdiomPhone},
		} {
			want := msgs.Start()
			if env.Idiom == core.IdiomPhone {
				want = msgs.TouchStart()
			}
			on := "mac"
			if env.Idiom == core.IdiomPhone {
				on = "phone"
			}
			t.Run(loc+"/"+on, func(t *testing.T) {
				app := start(t, &fakeKiosk{supported: env.Idiom == core.IdiomDesktop, volume: 0.5}, nsgotest.Environment(env))
				if !app.Exists("text:" + want) {
					t.Errorf("no %q on the setup", want)
				}
			})
		}
	}
}
