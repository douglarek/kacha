// Package record films the screen into a file.
package record

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
)

// ErrUnsupported reports that no recorder is here to film the screen: macOS
// films with screencapture, and a Wayland session through the portal of its
// desktop, whose stream GStreamer records.
var ErrUnsupported = errors.New("kacha: no recorder for the screen is here: macOS films with screencapture, a Wayland session through its portal with gst-launch-1.0")

// Options are what to film, and where the film goes.
type Options struct {
	// Rect is what to film, in the DIPs of the screen, empty for the whole
	// of it.
	Rect image.Rectangle
	// Display is the screen Rect is on, in DIPs, empty where the recorder
	// needs it not: a film of a Wayland session crops the stream of the
	// screen that the desktop shares to it.
	Display image.Rectangle
	// Seconds ends the film after this long, 0 for as long as it takes the
	// user to stop it.
	Seconds int
	// Clicks marks where the pointer clicked, in the film.
	Clicks bool
	// Out names the file to write, a file or a directory, empty for the
	// folder the captures go to.
	Out string
	// Started, when set, runs as the recorder rolls: what a caller says of a
	// film that has begun, before the recorder is known to be working.
	Started func()
}

// Logf, when set, records what the recorder could not do the way it meant
// to, for a launch that has no terminal to print to.
var Logf = func(format string, args ...any) {}

// Recorder is a film being made.
type Recorder struct {
	cmd *exec.Cmd
	// path is the file the caller asked for, and file the one the film is in
	// when it ends: the two differ where the film stays as the recorder
	// wrote it, in the QuickTime container, because it could not be moved
	// into an MP4.
	path, file string
	// done is closed once the film is over, by its own time limit or by a
	// stop, and err says how it went.
	done chan struct{}
	err  error
	// said is what the recorder wrote as it worked, which a failure tells.
	said bytes.Buffer
}

// Stop ends the film, which both recorders write out as an interrupt comes,
// and waits for the file.
func (r *Recorder) Stop() (string, error) {
	select {
	case <-r.done:
		return r.file, r.err
	default:
	}
	if r.cmd == nil || r.cmd.Process == nil {
		return "", ErrUnsupported
	}
	// A recorder that is already gone has its film written.
	if err := r.cmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return r.file, fmt.Errorf("kacha: stop recording: %w", err)
	}
	return r.Wait()
}

// Done reports when the film is over, by the time it was given or by Stop.
func (r *Recorder) Done() <-chan struct{} { return r.done }

// Wait waits for the film to end, and returns its file.
func (r *Recorder) Wait() (string, error) {
	<-r.done
	return r.file, r.err
}
