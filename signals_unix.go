//go:build unix

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// stopSignals end a film: the key that started it asks with SIGUSR1, which
// quits no app, and an interrupt or a termination is what a shell and the
// session of a desktop send.
var stopSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1}

// ownSession sends cmd to a session of its own, so that the key of a
// terminal and the going of kacha leave it alone.
func ownSession(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
