//go:build unix

package main

import (
	"os"
	"syscall"
)

// stopSignals end a film: the key that started it asks with SIGUSR1, which
// quits no app, and an interrupt or a termination is what a shell and the
// session of a desktop send.
var stopSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1}
