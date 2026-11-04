//go:build !darwin

package overlay

import "github.com/egoist/mygo"

// coverMenuBar does nothing: only macOS keeps an ordinary window below its
// menu bar.
func coverMenuBar(*mygo.Window) {}
