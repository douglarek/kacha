//go:build linux && (amd64 || arm64)

package shot

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/egoist/mygo"
)

// copyPNG puts data on the clipboard as PNG.
//
// A Wayland compositor serves the selection from the process that owns it, so
// a capture that a plain GTK write leaves behind dies with kacha wherever the
// desktop has no clipboard manager, as COSMIC has none: the paste after a
// capture comes up empty. wl-copy forks a process of its own to serve the
// selection, which outlives kacha.
func copyPNG(data []byte) error {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if err := wlCopy(data); err == nil {
			return nil
		}
	}
	return writeClipboard(data)
}

// writeClipboard copies through GTK, for a desktop where that lasts.
var writeClipboard = mygo.Clipboard.WriteImage

// wlCopy hands data to wl-copy.
var wlCopy = runWLCopy

// wlCopyFork is how long wl-copy may take to leave the process that serves
// the selection behind.
const wlCopyFork = 2 * time.Second

// runWLCopy starts wl-copy and waits for it to fork, which it does unless it
// serves the selection itself, and the wait ends when it stays.
func runWLCopy(data []byte) error {
	path, err := exec.LookPath("wl-copy")
	if err != nil {
		return err
	}
	cmd := exec.Command(path, "-t", "image/png")
	cmd.Stdin = bytes.NewReader(data)
	// A pipe for the output would outlive the fork, as the process that
	// serves the selection holds its end: Wait would then block until that
	// process ends, freezing the window, and a compositor drops a client
	// that stops answering its events. Nothing here reads the output, so
	// both go to the null device.
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("kacha: wl-copy: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("kacha: wl-copy: %w", err)
		}
		return nil
	case <-time.After(wlCopyFork):
		return nil
	}
}
