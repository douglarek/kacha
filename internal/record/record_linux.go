//go:build linux

package record

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/douglarek/kacha/internal/shot"
)

// encoder is a codec a film is tried with, and the container it goes in.
type encoder struct {
	// codec is the GStreamer element that encodes the stream.
	codec string
	// parse is the element that shapes the encoded stream for the muxer, ""
	// where the muxer takes it as it comes.
	parse string
	// mux is the element that writes the container.
	mux string
	// ext is the extension of the file that container is named with.
	ext string
	// props returns what the codec is asked for a film of size: what it
	// encodes at, and what it films, which its defaults get wrong.
	props func(size image.Point) []string
}

// mp4 returns the encoder of a film in the MP4 container, which a file is
// named with ext.
func mp4(ext string) encoder {
	return encoder{
		codec: "openh264enc", parse: "h264parse", mux: "mp4mux", ext: ext,
		// OpenH264 encodes 128 kbit/s and takes the film for one of a camera
		// unless told otherwise, which no screen reads at.
		props: func(size image.Point) []string {
			return []string{
				fmt.Sprintf("bitrate=%d", bitrateFor(size)),
				"rate-control=bitrate",
				"usage-type=screen",
			}
		},
	}
}

// x264 returns the encoder of a film in the MP4 container of the GStreamer
// of Debian and Arch Linux, which carries x264 where Fedora carries OpenH264.
func x264(ext string) encoder {
	return encoder{
		codec: "x264enc", parse: "h264parse", mux: "mp4mux", ext: ext,
		// The encoder keeps a quality rather than a bitrate: 20 of 51 holds
		// the text of a screen. It is left to buffer its frames, which
		// trades the latency of a film for its picture.
		props: func(image.Point) []string {
			return []string{
				"pass=qual",
				"quantizer=20",
				"speed-preset=veryfast",
			}
		},
	}
}

// webm returns the encoder of a film in the WebM container.
func webm() encoder {
	return encoder{
		codec: "vp9enc", mux: "matroskamux", ext: ".webm",
		// VP9 takes a quarter of a megabit a second, a second to encode each
		// frame and 25 frames of lag, which films no screen as it runs.
		props: func(size image.Point) []string {
			return []string{
				fmt.Sprintf("target-bitrate=%d", bitrateFor(size)),
				"deadline=1",
				"cpu-used=6",
				"lag-in-frames=0",
			}
		},
	}
}

// encoders are the codecs a film is tried with, in the order that keeps it
// playable: the H.264 of the GStreamer of Fedora, the H.264 of the one of
// Debian and Arch Linux, and the VP9 every GStreamer carries.
var encoders = []encoder{mp4(".mp4"), x264(".mp4"), webm()}

// bitrateFor returns what a film of size is encoded at: four bits a second
// for each pixel of a frame, which keeps the text of a desktop readable, and
// what a disk of a laptop takes, within the two ends of what films are made
// at.
func bitrateFor(size image.Point) int {
	pixels := size.X * size.Y
	rate := pixels * 4
	if rate < 4_000_000 {
		return 4_000_000
	}
	if rate > 48_000_000 {
		return 48_000_000
	}
	return rate
}

// settle is how long a recorder has to fail for want of a codec before it is
// taken as recording.
const settle = 2 * time.Second

// pickTimeout is how long the user has to pick what to share, at the picker
// of the desktop.
const pickTimeout = 2 * time.Minute

// Supported reports whether this session shares its screen, which the portal
// of a Wayland desktop does, and records it, which GStreamer does.
func Supported() bool { return gstLaunch() != "" && portalAvailable() }

// gstLaunch returns the path of gst-launch-1.0, "" without one.
func gstLaunch() string {
	path, err := exec.LookPath("gst-launch-1.0")
	if err != nil {
		return ""
	}
	return path
}

// Start films what the desktop shares: it asks the portal for a stream of a
// screen or a window, which the desktop's own picker names, and records it
// with GStreamer, which writes the film as H.264 when it can and VP9 where
// it cannot.
func Start(o Options) (*Recorder, error) {
	gst := gstLaunch()
	if gst == "" {
		return nil, fmt.Errorf("%w: install GStreamer, whose pipewiresrc, videocrop, mp4mux, h264parse and openh264 or x264 record a screen", ErrUnsupported)
	}
	if o.Rect != (image.Rectangle{}) && o.Display == (image.Rectangle{}) {
		// The desktop shares a screen whole: the area is cut out of it, which
		// needs to know which screen that area is on.
		Logf("record: no display of the area is known, so the whole screen the desktop shares is filmed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), pickTimeout)
	shared, err := openShare(ctx)
	cancel()
	if err != nil {
		return nil, err
	}

	var said []string
	for _, c := range encodersFor(o.Out) {
		r, err := start(o, gst, c, shared)
		if err == nil {
			if o.Seconds > 0 {
				time.AfterFunc(time.Duration(o.Seconds)*time.Second, func() { _, _ = r.Stop() })
			}
			return r, nil
		}
		said = append(said, err.Error())
	}
	shared.Close()
	return nil, fmt.Errorf("kacha: record: %s", strings.Join(said, "; "))
}

// start films with one codec, and reports the recorder broken when it stops
// before it records anything: a GStreamer without the element of the codec
// says so at once.
func start(o Options, gst string, c encoder, shared *share) (*Recorder, error) {
	file, err := shot.Path(o.Out, c.ext)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return nil, fmt.Errorf("kacha: record: %w", err)
	}
	cmd := exec.Command(gst, pipeline(shared.node, shared.size, o, c, file)...)
	// The connection to the stream reaches the recorder as the third
	// descriptor of the process, which pipewiresrc opens it by.
	cmd.ExtraFiles = []*os.File{shared.conn}
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
			return nil, fmt.Errorf("%s: %v: %s", c.codec, err, said)
		}
		return nil, fmt.Errorf("%s: %v", c.codec, err)
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
		shared.Close()
	}()
	return r, nil
}

// pipeline returns the command that records the stream of node into file: -e
// writes the film out as it is stopped, which is how the key that started it
// ends it.
func pipeline(node uint32, size image.Point, o Options, c encoder, file string) []string {
	args := []string{
		"-e",
		"pipewiresrc", "fd=3", fmt.Sprintf("path=%d", node),
		"!", "videoconvert",
	}
	left, top, right, bottom := crop(o, size)
	if left+top+right+bottom > 0 {
		args = append(args, "!", "videocrop",
			fmt.Sprintf("left=%d", left), fmt.Sprintf("top=%d", top),
			fmt.Sprintf("right=%d", right), fmt.Sprintf("bottom=%d", bottom))
	}
	// What the codec encodes: the area kept, in the pixels of the stream.
	kept := image.Pt(size.X-left-right, size.Y-top-bottom)
	args = append(args, "!", c.codec)
	if c.props != nil {
		args = append(args, c.props(kept)...)
	}
	if c.parse != "" {
		args = append(args, "!", c.parse)
	}
	return append(args,
		"!", c.mux,
		"!", "filesink", "location="+file,
	)
}

// crop returns how much of a stream of size is cut from each side to keep the
// area of o, which is in the DIPs of the display it is on: the stream holds
// the screen in the pixels the desktop gives it, so the area is scaled to
// them. Nothing is cut where the film is of the whole screen, or where the
// size of the stream is unknown.
func crop(o Options, size image.Point) (left, top, right, bottom int) {
	if o.Rect.Empty() || o.Display.Empty() || size.X <= 0 || size.Y <= 0 {
		return 0, 0, 0, 0
	}
	scaleX := float64(size.X) / float64(o.Display.Dx())
	scaleY := float64(size.Y) / float64(o.Display.Dy())
	left = int(math.Round(float64(o.Rect.Min.X-o.Display.Min.X) * scaleX))
	top = int(math.Round(float64(o.Rect.Min.Y-o.Display.Min.Y) * scaleY))
	right = size.X - int(math.Round(float64(o.Rect.Max.X-o.Display.Min.X)*scaleX))
	bottom = size.Y - int(math.Round(float64(o.Rect.Max.Y-o.Display.Min.Y)*scaleY))
	// An encoder takes no odd edge, and a crop of nothing is no crop.
	left, top, right, bottom = left&^1, top&^1, right&^1, bottom&^1
	if left < 0 {
		left = 0
	}
	if top < 0 {
		top = 0
	}
	if right < 0 {
		right = 0
	}
	if bottom < 0 {
		bottom = 0
	}
	if left+right >= size.X || top+bottom >= size.Y {
		return 0, 0, 0, 0
	}
	return left, top, right, bottom
}

// encodersFor returns the codecs to try for out: a file named with an
// extension takes what that extension holds, and a folder takes them all.
func encodersFor(out string) []encoder {
	switch ext := strings.ToLower(filepath.Ext(out)); ext {
	case "":
		return encoders
	case ".webm":
		return []encoder{webm()}
	case ".mp4", ".mov":
		return []encoder{mp4(ext), x264(ext)}
	}
	return encoders
}
