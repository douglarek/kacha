package record

import (
	"image"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlagsForAFilm(t *testing.T) {
	tests := []struct {
		name string
		o    Options
		want string
	}{
		{"the whole screen", Options{}, "-v"},
		{"a rectangle", Options{Rect: image.Rect(10, 20, 330, 220)}, "-v -R 10,20,320,200"},
		{"clicks", Options{Clicks: true}, "-v -k"},
		{"a time limit", Options{Seconds: 5}, "-v -V 5"},
		{"everything", Options{Rect: image.Rect(0, 0, 100, 50), Clicks: true, Seconds: 2}, "-v -R 0,0,100,50 -k -V 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Join(flags(tt.o), " "); got != tt.want {
				t.Errorf("flags = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFiles(t *testing.T) {
	dir := t.TempDir()

	// A folder takes the name of the moment, an MP4 beside a QuickTime film
	// of the same name for the recorder to write.
	path, raw, err := files(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir || filepath.Ext(path) != ".mp4" {
		t.Errorf("path = %q, want an .mp4 in %q", path, dir)
	}
	if filepath.Ext(raw) != ".mov" || strings.TrimSuffix(raw, ".mov") != strings.TrimSuffix(path, ".mp4") {
		t.Errorf("raw = %q, want the .mov of %q", raw, path)
	}

	// A film asked for as a QuickTime film is written as one.
	path, raw, err = files(filepath.Join(dir, "film.mov"))
	if err != nil {
		t.Fatal(err)
	}
	if path != raw || filepath.Ext(path) != ".mov" {
		t.Errorf("path = %q, raw = %q, want the .mov alone", path, raw)
	}

	// Another name is no film to write: it becomes an MP4.
	path, raw, err = files(filepath.Join(dir, "film.avi"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".mp4" || filepath.Ext(raw) != ".mov" {
		t.Errorf("path = %q, raw = %q, want an .mp4 and its .mov", path, raw)
	}
}
