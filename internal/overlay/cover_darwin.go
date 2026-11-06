//go:build darwin

package overlay

import (
	"github.com/douglarek/kacha/internal/macwin"
	"github.com/egoist/mygo"
)

// coverMenuBar lifts the window above the menu bar, so that it covers the
// whole display rather than the work area below it, and keeps the keys, so
// that Escape leaves a capture started from anywhere.
func coverMenuBar(w *mygo.Window) { macwin.CoverMenuBar(w) }

// watchEscape takes Escape from the keyboard itself while the overlay is up,
// which a window the menu bar's app never gave the keys leaves the capture
// by. It returns nil when the shortcut could not be registered, and the
// overlay then leaves by the key alone.
func watchEscape(leave func()) func() {
	if err := mygo.GlobalShortcut.Register("Escape", leave); err != nil {
		Logf("overlay: no global escape: %v", err)
		return nil
	}
	return func() { mygo.GlobalShortcut.Unregister("Escape") }
}
