//go:build !darwin

package capture

import (
	"context"
	"fmt"
	"os"
)

// Identity is what a capture needs to know about the running copy.
type Identity struct {
	Path    string
	Bundle  string
	Host    string
	Allowed bool
}

// Identify describes the running copy.
func Identify() Identity {
	exe, err := os.Executable()
	if err != nil {
		exe = "kacha"
	}
	return Identity{Path: exe, Allowed: true}
}

// WaitAllowed reports at once that no platform but macOS asks for
// permission to record the screen.
func WaitAllowed(context.Context) bool { return false }

// Allowed reports that no platform but macOS asks for permission to record
// the screen.
func Allowed() bool { return true }

// RequestAccess reports that only macOS asks the user for permission to
// record the screen.
func RequestAccess() (bool, error) {
	return true, fmt.Errorf("kacha: only macOS asks for permission to record the screen")
}
