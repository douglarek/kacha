// Package record films the screen into a file.
package record

import (
	"bytes"
	"errors"
	"image"
	"os/exec"
)

// ErrUnsupported reports that this system has no recorder the app can drive.
var ErrUnsupported = errors.New("kacha: recording needs macOS, which films the screen of its own")

// Options are what to film, and where the film goes.
type Options struct {
	// Rect is what to film, in the DIPs of the screen, empty for the whole
	// of it.
	Rect image.Rectangle
	// Seconds ends the film after this long, 0 for as long as it takes the
	// user to stop it.
	Seconds int
	// Clicks marks where the pointer clicked, in the film.
	Clicks bool
	// Out names the file to write, a file or a directory, empty for the
	// folder the captures go to.
	Out string
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

// Done reports when the film is over, by the time it was given or by Stop.
func (r *Recorder) Done() <-chan struct{} { return r.done }

// Wait waits for the film to end, and returns its file.
func (r *Recorder) Wait() (string, error) {
	<-r.done
	return r.file, r.err
}
