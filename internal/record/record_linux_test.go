//go:build linux

package record

import (
	"image"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlagsForAFilm(t *testing.T) {
	tests := []struct {
		name  string
		o     Options
		codec string
		want  string
	}{
		{"the whole screen", Options{}, "libx264", "-c libx264"},
		{"a rectangle", Options{Rect: image.Rect(10, 20, 330, 220)}, "libvpx-vp9", "-c libvpx-vp9 -g 10,20 320x200"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Join(flags(tt.o, tt.codec), " "); got != tt.want {
				t.Errorf("flags = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodersForTheFile(t *testing.T) {
	dir := t.TempDir()

	// A folder takes every codec, the first that records winning.
	var names []string
	for _, c := range encodersFor(dir) {
		names = append(names, c.name)
	}
	if got := strings.Join(names, ","); got != "libx264,libvpx-vp9,libopenh264" {
		t.Errorf("encoders = %q, want the three in that order", got)
	}

	// A file named with an extension takes what that extension holds.
	if got := encodersFor(filepath.Join(dir, "film.webm")); len(got) != 1 || got[0].name != "libvpx-vp9" {
		t.Errorf("a .webm takes %+v, want libvpx-vp9 alone", got)
	}
	if got := encodersFor(filepath.Join(dir, "film.mov")); len(got) != 2 || got[0].ext != ".mov" {
		t.Errorf("a .mov takes %+v, want the codecs of QuickTime", got)
	}
	if got := encodersFor(filepath.Join(dir, "film.mp4")); len(got) != 2 || got[0].name != "libx264" {
		t.Errorf("an .mp4 takes %+v, want x264 first", got)
	}
}
