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
