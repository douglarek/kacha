//go:build windows

package main

import "os"

// stopSignals end a film: Windows has the interrupt alone.
var stopSignals = []os.Signal{os.Interrupt}
