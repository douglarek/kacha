//go:build darwin

package capture

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"strings"
)

// screencaptureBackend captures through the screencapture tool of macOS.
type screencaptureBackend struct{ path string }

// Name identifies the backend.
func (screencaptureBackend) Name() string { return "screencapture" }

// Capture takes the whole screen, without the pointer.
func (b screencaptureBackend) Capture(ctx context.Context) (image.Image, error) {
	file, err := os.CreateTemp("", "kacha-*.png")
	if err != nil {
		return nil, fmt.Errorf("capture: %w", err)
	}
	name := file.Name()
	file.Close()
	defer os.Remove(name)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, b.path, "-x", "-t", "png", name)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "could not create image") {
			return nil, accessError()
		}
		return nil, fmt.Errorf("capture: screencapture: %w: %s", err, msg)
	}
	out, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("capture: %w", err)
	}
	defer out.Close()
	return decode(out)
}
