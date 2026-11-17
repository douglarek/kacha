//go:build darwin

package record

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/douglarek/kacha/internal/shot"
)

// Supported reports that this system films the screen for the app.
func Supported() bool { return true }

// Start films the screen with screencapture, the recorder macOS has, and
// returns once it is rolling.
func Start(o Options) (*Recorder, error) {
	path, raw, err := files(o.Out)
	if err != nil {
		return nil, err
	}
	// screencapture writes nowhere it finds no folder, and the captures go
	// to one that a first run has not made yet.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("kacha: record: %w", err)
	}
	args := append([]string{"-x"}, flags(o)...)
	cmd := exec.Command("screencapture", append(args, raw)...)
	r := &Recorder{cmd: cmd, path: path, done: make(chan struct{})}
	cmd.Stderr = &r.said
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("kacha: record: %w", err)
	}
	go func() {
		defer close(r.done)
		if err := cmd.Wait(); err != nil {
			said := strings.TrimSpace(r.said.String())
			if said != "" {
				r.err = fmt.Errorf("kacha: record: %w: %s", err, said)
			} else {
				r.err = fmt.Errorf("kacha: record: %w", err)
			}
			// A film that broke off is no film, and the folder it was in
			// would keep half of one.
			os.Remove(raw)
			return
		}
		r.file = raw
		if raw == path {
			return
		}
		if err := remux(raw, path); err != nil {
			// The film is there, as the recorder wrote it.
			Logf("record: %v, the film stays at %s", err, raw)
			return
		}
		os.Remove(raw)
		r.file = path
	}()
	return r, nil
}

// files returns the file the caller asked for, and the one screencapture
// writes first: a QuickTime film under the same name, which avconvert moves
// into the MP4 that plays wherever a film is played.
func files(out string) (path, raw string, err error) {
	path, err = shot.Path(out, ".mp4")
	if err != nil {
		return "", "", err
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	switch strings.ToLower(ext) {
	case ".mov":
		// A film asked for as a QuickTime film stays one.
		return path, path, nil
	case ".mp4":
	default:
		// No other name is a film this writes.
		path = base + ".mp4"
	}
	return path, base + ".mov", nil
}

// remux moves the stream of the film at raw into an MP4 at path, which is
// avconvert's work: it carries the H.264 stream as it is, and no bit of it is
// lost.
func remux(raw, path string) error {
	args := []string{"--source", raw, "--output", path, "--preset", "PresetPassthrough"}
	if said, err := exec.Command("avconvert", args...).CombinedOutput(); err != nil {
		if msg := strings.TrimSpace(string(said)); msg != "" {
			return fmt.Errorf("kacha: record: avconvert: %w: %s", err, msg)
		}
		return fmt.Errorf("kacha: record: avconvert: %w", err)
	}
	return nil
}

// flags returns what a film of o asks screencapture for, the file aside.
func flags(o Options) []string {
	args := []string{"-v"}
	if r := o.Rect; !r.Empty() {
		args = append(args, "-R", fmt.Sprintf("%d,%d,%d,%d", r.Min.X, r.Min.Y, r.Dx(), r.Dy()))
	}
	if o.Clicks {
		args = append(args, "-k")
	}
	if o.Seconds > 0 {
		args = append(args, "-V", strconv.Itoa(o.Seconds))
	}
	return args
}
