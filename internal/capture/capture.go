// Package capture takes screenshots from the desktop.
package capture

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"
	"os"

	_ "image/jpeg"
	_ "image/png"
)

var (
	// ErrUnsupported reports that this session offers no capture backend.
	ErrUnsupported = errors.New("kacha: no screen capture backend for this session")
	// ErrDenied reports that the desktop refused the capture.
	ErrDenied = errors.New("kacha: screen capture was denied")
)

// Window is a window on screen, in the screen's DIPs, topmost first in a
// list.
type Window struct {
	Rect  image.Rectangle
	App   string
	Title string
}

// Backend captures the screen.
type Backend interface {
	// Name identifies the protocol the backend speaks.
	Name() string
	// Capture returns the screen as it is now.
	Capture(ctx context.Context) (image.Image, error)
}

// Grab captures the screen with the backend the session offers, or reads
// the image KACHA_SOURCE names, which shows the overlay of a machine that
// cannot capture.
func Grab(ctx context.Context) (image.Image, error) {
	if source := os.Getenv("KACHA_SOURCE"); source != "" {
		file, err := os.Open(source)
		if err != nil {
			return nil, fmt.Errorf("capture: %w", err)
		}
		defer file.Close()
		return decode(file)
	}
	b, err := Detect()
	if err != nil {
		return nil, err
	}
	return b.Capture(ctx)
}

// Crop returns the part of img inside r.
func Crop(img image.Image, r image.Rectangle) *image.RGBA {
	r = r.Intersect(img.Bounds())
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), img, r.Min, draw.Src)
	return out
}

// decode reads the PNG or JPEG a backend produced.
func decode(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("capture: decode screenshot: %w", err)
	}
	return img, nil
}
