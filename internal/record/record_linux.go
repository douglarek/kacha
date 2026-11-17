//go:build linux

package record

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/douglarek/kacha/internal/shot"
)

// encoder is a codec a film is tried with, and the file it goes in.
type encoder struct {
	name string
	ext  string
}

// encoders are the codecs a film is tried with, in the order that keeps it
// playable: the H.264 of an ffmpeg that carries it, and the VP9 that the
// ffmpeg Fedora ships carries alone.
var encoders = []encoder{
	{"libx264", ".mp4"},
	{"libvpx-vp9", ".webm"},
	{"libopenh264", ".mp4"},
}

// settle is how long a recorder has to fail for want of a codec before it is
// taken as recording.
const settle = 700 * time.Millisecond

// Supported reports whether wf-recorder, the recorder of a Wayland session,
// is here to film.
func Supported() bool { return wfRecorder() != "" }

// wfRecorder returns the path of wf-recorder, "" without one.
func wfRecorder() string {
	path, err := exec.LookPath("wf-recorder")
	if err != nil {
		return ""
	}
	return path
}

// Start films the screen with wf-recorder, and returns once it is rolling.
func Start(o Options) (*Recorder, error) {
	wf := wfRecorder()
	if wf == "" {
		return nil, fmt.Errorf("%w: install wf-recorder", ErrUnsupported)
	}
	var said []string
	for _, c := range encodersFor(o.Out) {
		r, err := start(o, wf, c)
		if err == nil {
			if o.Seconds > 0 {
				time.AfterFunc(time.Duration(o.Seconds)*time.Second, func() { _, _ = r.Stop() })
			}
			return r, nil
		}
		said = append(said, err.Error())
	}
	return nil, fmt.Errorf("kacha: record: %s", strings.Join(said, "; "))
}

// start films with one codec, and reports the recorder broken when it stops
// before it records anything: an ffmpeg without the codec says so at once.
func start(o Options, wf string, c encoder) (*Recorder, error) {
	file, err := shot.Path(o.Out, c.ext)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return nil, fmt.Errorf("kacha: record: %w", err)
	}
	cmd := exec.Command(wf, append(flags(o, c.name), "-f", file)...)
	r := &Recorder{cmd: cmd, path: file, file: file, done: make(chan struct{})}
	cmd.Stderr = &r.said
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("kacha: record: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		os.Remove(file)
		if said := strings.TrimSpace(r.said.String()); said != "" {
			return nil, fmt.Errorf("%s: %v: %s", c.name, err, said)
		}
		return nil, fmt.Errorf("%s: %v", c.name, err)
	case <-time.After(settle):
	}
	go func() {
		defer close(r.done)
		if err := <-exited; err != nil {
			if said := strings.TrimSpace(r.said.String()); said != "" {
				r.err = fmt.Errorf("kacha: record: %w: %s", err, said)
			} else {
				r.err = fmt.Errorf("kacha: record: %w", err)
			}
			// A film that broke off is no film.
			os.Remove(r.file)
		}
	}()
	return r, nil
}

// encodersFor returns the codecs to try for out: a file named with an
// extension takes what that extension holds, and a folder takes them all.
func encodersFor(out string) []encoder {
	switch ext := strings.ToLower(filepath.Ext(out)); ext {
	case "":
		return encoders
	case ".webm":
		return []encoder{{"libvpx-vp9", ".webm"}}
	case ".mp4", ".mov":
		return []encoder{{"libx264", ext}, {"libopenh264", ext}}
	case ".mkv":
		return encoders
	}
	return encoders
}

// flags returns what a film of o asks wf-recorder for, the file aside. The
// clicks a film marks on macOS have no mark here.
func flags(o Options, codec string) []string {
	args := []string{"-c", codec}
	if r := o.Rect; !r.Empty() {
		args = append(args, "-g", fmt.Sprintf("%d,%d %dx%d", r.Min.X, r.Min.Y, r.Dx(), r.Dy()))
	}
	return args
}
