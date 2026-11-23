//go:build ios

package kiosk

import (
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// gameController loads GameController, whose GCKeyboard knows whether a
// physical keyboard is attached; false should it not load.
var gameController = sync.OnceValue(func() bool {
	_, err := purego.Dlopen("/System/Library/Frameworks/GameController.framework/GameController", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	return err == nil && objc.GetClass("GCKeyboard") != 0
})

// Keyboard reports whether a physical keyboard is attached to the iPhone
// or iPad.
func Keyboard() bool {
	if !gameController() {
		return false
	}
	return objc.ID(objc.GetClass("GCKeyboard")).Send(objc.RegisterName("coalescedKeyboard")) != 0
}
