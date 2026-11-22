package main

import (
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// turnAnimation eases the layout over when the screen turns between
// portrait and landscape: keys and columns glide and resize to their new
// places, and what comes or goes fades, instead of jumping.
var turnAnimation = core.LayoutAnimation{
	Create: &core.LayoutTransition{Curve: core.LayoutEaseInEaseOut, Duration: 300 * time.Millisecond},
	Update: &core.LayoutTransition{Curve: core.LayoutSpring, Damping: 0.8, Duration: 450 * time.Millisecond},
	Delete: &core.LayoutTransition{Curve: core.LayoutEaseInEaseOut, Duration: 200 * time.Millisecond},
}

// turned reports whether a view that measured was and now measures now has
// turned between portrait and landscape. The first measure is no turn.
func turned(was, now core.Size) bool {
	if was.Width == 0 || was.Height == 0 || now.Width == 0 || now.Height == 0 {
		return false
	}
	return (was.Width > was.Height) != (now.Width > now.Height)
}

// animateTurn animates the next commit's layout if the view turned.
func animateTurn(cx *core.Scope, was, now core.Size) {
	if turned(was, now) {
		cx.AnimateLayout(turnAnimation)
	}
}
