//go:build linux

package record

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"testing"
)

func TestPipelineOfAFilm(t *testing.T) {
	whole := strings.Join(pipeline(7, image.Pt(1920, 1080), Options{}, mp4(".mp4"), "/tmp/film.mp4"), " ")
	// 1920x1080 at four bits a second for each pixel of a frame.
	want := "-e pipewiresrc fd=3 path=7 ! videoconvert ! openh264enc bitrate=8294400 rate-control=bitrate usage-type=screen ! h264parse ! mp4mux ! filesink location=/tmp/film.mp4"
	if whole != want {
		t.Errorf("pipeline = %q, want %q", whole, want)
	}

	// The connection reaches the recorder as its third descriptor, which is
	// what pipewiresrc opens the stream through.
	if !strings.Contains(whole, "fd=3") {
		t.Errorf("pipeline = %q, want the stream on fd 3", whole)
	}

	// The H.264 of Debian and Arch Linux keeps a quality, not a bitrate.
	better := strings.Join(pipeline(7, image.Pt(1920, 1080), Options{}, x264(".mp4"), "/tmp/film.mp4"), " ")
	if !strings.Contains(better, "x264enc pass=qual quantizer=20 speed-preset=veryfast") {
		t.Errorf("pipeline = %q, want the quality of x264", better)
	}

	// An area of the screen is cut out of the stream of it.
	area := Options{
		Rect:    image.Rect(1100, 100, 1500, 700),
		Display: image.Rect(1000, 0, 2000, 1000),
	}
	got := strings.Join(pipeline(7, image.Pt(2000, 2000), area, encoders[0], "/tmp/a.mp4"), " ")
	if !strings.Contains(got, "videocrop left=200 top=200 right=1000 bottom=600") {
		t.Errorf("pipeline = %q, want the area cut out of the stream", got)
	}
	// What is encoded is the area kept, not the whole stream: 800x1200 at
	// four bits a second for each pixel of a frame, which no film goes under.
	if !strings.Contains(got, "bitrate=4000000") {
		t.Errorf("pipeline = %q, want the bitrate of the area kept", got)
	}
}

func TestCropOfTheArea(t *testing.T) {
	tests := []struct {
		name       string
		o          Options
		size       image.Point
		wantString string
	}{
		{"the whole screen", Options{}, image.Pt(1920, 1080), "0,0,0,0"},
		{"an area at the scale of the screen", Options{
			Rect:    image.Rect(10, 20, 330, 220),
			Display: image.Rect(0, 0, 1920, 1080),
		}, image.Pt(1920, 1080), "10,20,1590,860"},
		{"an area of a screen that is filmed at twice its DIPs", Options{
			Rect:    image.Rect(10, 20, 330, 220),
			Display: image.Rect(0, 0, 1920, 1080),
		}, image.Pt(3840, 2160), "20,40,3180,1720"},
		{"an area of another screen", Options{
			Rect:    image.Rect(1930, 20, 2250, 220),
			Display: image.Rect(1920, 0, 3840, 1080),
		}, image.Pt(1920, 1080), "10,20,1590,860"},
		{"no size of the stream", Options{
			Rect:    image.Rect(10, 20, 330, 220),
			Display: image.Rect(0, 0, 1920, 1080),
		}, image.Point{}, "0,0,0,0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left, top, right, bottom := crop(tt.o, tt.size)
			got := fmt.Sprintf("%d,%d,%d,%d", left, top, right, bottom)
			if got != tt.wantString {
				t.Errorf("crop = %s, want %s", got, tt.wantString)
			}
		})
	}
}

func TestEncodersForTheFile(t *testing.T) {
	dir := t.TempDir()

	// A folder takes every codec, the first that records winning.
	var names []string
	for _, c := range encodersFor(dir) {
		names = append(names, c.codec)
	}
	if got := strings.Join(names, ","); got != "openh264enc,x264enc,vp9enc" {
		t.Errorf("encoders = %q, want the three in that order", got)
	}

	// A file named with an extension takes what that extension holds.
	if got := encodersFor(filepath.Join(dir, "film.webm")); len(got) != 1 || got[0].codec != "vp9enc" || got[0].mux != "matroskamux" {
		t.Errorf("a .webm takes %+v, want VP9 alone", got)
	}
	if got := encodersFor(filepath.Join(dir, "film.mp4")); len(got) != 2 || got[0].codec != "openh264enc" || got[1].codec != "x264enc" {
		t.Errorf("an .mp4 takes %+v, want the two encoders of H.264", got)
	}
	if got := encodersFor(filepath.Join(dir, "film.mov")); len(got) != 2 || got[1].ext != ".mov" {
		t.Errorf("a .mov takes %+v, want QuickTime", got)
	}
}
