//go:build darwin && !ios

package kiosk

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/ebitengine/purego"
)

func init() { runtime.LockOSThread() } // TestMain stays on the main thread

// TestMain serves the main run loop, as the app's AppKit does, while the
// tests run: the tap's callback runs there.
func TestMain(m *testing.M) {
	load()
	var runInMode func(mode uintptr, seconds float64, returnAfterSource bool) int32
	purego.RegisterLibFunc(&runInMode, coreFoundation, "CFRunLoopRunInMode")
	defaultMode := symValue(coreFoundation, "kCFRunLoopDefaultMode")
	code := make(chan int, 1)
	go func() { code <- m.Run() }()
	for {
		select {
		case c := <-code:
			os.Exit(c)
		default:
			runInMode(defaultMode, 0.05, false)
		}
	}
}

// TestTapSwallowsKeys locks the real keyboard for under a second, posts a
// key as the keyboard would and checks the lock heard it. It takes the
// keyboard of whoever runs it, so it runs only with BABYPIANO_LOCK_TEST=1,
// and needs the Accessibility permission for the test's process.
func TestTapSwallowsKeys(t *testing.T) {
	if os.Getenv("BABYPIANO_LOCK_TEST") != "1" {
		t.Skip("set BABYPIANO_LOCK_TEST=1 to lock this Mac's keyboard for a moment")
	}
	keys := make(chan Key, 4)
	if err := Start(Options{}, func(k Key) { keys <- k }); err != nil {
		t.Fatal(err)
	}
	safety := time.AfterFunc(2*time.Second, Stop) // never leave the keyboard locked
	defer safety.Stop()
	defer Stop()

	var createKey func(source uintptr, code uint16, down bool) uintptr
	var post func(tap uint32, event uintptr)
	purego.RegisterLibFunc(&createKey, appServices, "CGEventCreateKeyboardEvent")
	purego.RegisterLibFunc(&post, appServices, "CGEventPost")
	const hidEventTap, keyA = 0, 0
	for _, down := range []bool{true, false} {
		e := createKey(0, keyA, down)
		post(hidEventTap, e)
		cfRelease(e)
	}
	select {
	case k := <-keys:
		if k.Name != "a" {
			t.Errorf("heard %+v, want a", k)
		}
	case <-time.After(time.Second):
		t.Fatal("the lock heard no key")
	}
	if !lockHeld() {
		t.Error("the tap went off")
	}
}

func lockHeld() bool {
	lock.Lock()
	defer lock.Unlock()
	return lock.tap != 0 && cgEventTapIsEnabled(lock.tap)
}
