//go:build !darwin

package overlay

import "github.com/egoist/mygo"

// coverMenuBar does nothing: only macOS keeps an ordinary window below its
// menu bar.
func coverMenuBar(*mygo.Window) {}

// watchEscape does nothing outside macOS: a window takes the keys a click on
// it would.
func watchEscape(func()) func() { return nil }
