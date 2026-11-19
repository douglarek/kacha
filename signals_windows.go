//go:build windows

package main

import (
	"os"
	"os/exec"
)

// stopSignals end a film: Windows has the interrupt alone.
var stopSignals = []os.Signal{os.Interrupt}

// ownSession leaves cmd as it is: Windows has no session to send it to.
func ownSession(cmd *exec.Cmd) {}
