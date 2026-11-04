//go:build darwin

package capture

import (
	"fmt"
	"os/exec"
)

// Detect returns the screencapture tool of macOS.
func Detect() (Backend, error) {
	path, err := exec.LookPath("screencapture")
	if err != nil {
		return nil, fmt.Errorf("%w: screencapture is not in PATH", ErrUnsupported)
	}
	return screencaptureBackend{path: path}, nil
}
