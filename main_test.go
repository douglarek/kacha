package main

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

func TestStripsInFindsTheMenuBar(t *testing.T) {
	d := mygo.Display{
		Bounds:   mygo.Rectangle{Width: 1710, Height: 1107},
		WorkArea: mygo.Rectangle{Y: 34, Width: 1710, Height: 1073},
	}
	got := stripsIn(d, image.Rect(0, 0, 1710, 1107))
	if len(got) != 1 {
		t.Fatalf("strips = %d, want the menu bar alone", len(got))
	}
	if got[0].App != stripLabel(true) {
		t.Errorf("strip = %q, want the name of the menu bar", got[0].App)
	}
	if want := image.Rect(0, 0, 1710, 34); got[0].Rect != want {
		t.Errorf("rect = %v, want %v", got[0].Rect, want)
	}
	if got[0].Round {
		t.Error("the menu bar has no rounded corners")
	}
}

func TestStripsInFindsTheDock(t *testing.T) {
	d := mygo.Display{
		Bounds:   mygo.Rectangle{Width: 1710, Height: 1107},
		WorkArea: mygo.Rectangle{Y: 34, Width: 1710, Height: 990},
	}
	got := stripsIn(d, image.Rect(0, 0, 1710, 1107))
	if len(got) != 2 {
		t.Fatalf("strips = %d, want the menu bar and the Dock", len(got))
	}
	dock := got[1]
	if dock.App != stripLabel(false) {
		t.Errorf("strip = %q, want the name of the Dock", dock.App)
	}
	if want := image.Rect(0, 1024, 1710, 1107); dock.Rect != want {
		t.Errorf("dock = %v, want %v", dock.Rect, want)
	}
}

func TestRememberingWhereAFilmWent(t *testing.T) {
	if got := lastFilm(); got != "" {
		t.Fatalf("a film is remembered before one is made: %q", got)
	}
	rememberFilm("/home/me/Shots/film.mp4")
	if got := lastFilm(); got != "/home/me/Shots/film.mp4" {
		t.Errorf("last film = %q, want the one just made", got)
	}
}

func TestFilmGlyphIsARedDot(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(filmGlyph()))
	if err != nil {
		t.Fatalf("the icon of a film is no PNG: %v", err)
	}
	if got := img.Bounds().Dx(); got != 32 {
		t.Errorf("the icon is %d pixels wide, want the 32 of a panel icon", got)
	}
	// A red dot, which is what a recorder shows, with the white square of the
	// stop button in it.
	if r, g, b, a := img.At(16, 7).RGBA(); a == 0 || r>>8 < 200 || g>>8 > 100 || b>>8 > 100 {
		t.Errorf("the dot is %v, want the red of a recording", img.At(16, 7))
	}
	if r, g, b, _ := img.At(16, 16).RGBA(); r>>8 < 200 || g>>8 < 200 || b>>8 < 200 {
		t.Errorf("the middle is %v, want the white square of a stop", img.At(16, 16))
	}
	// And nothing at the corner, as the icon of a panel has no background.
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("the corner is drawn, want it clear")
	}
}

func TestShowItemsAsksForTheFile(t *testing.T) {
	got := strings.Join(showItemsArgs("/home/me/Shots/a b.mp4"), " ")
	want := `call --session --dest org.freedesktop.FileManager1 ` +
		`--object-path /org/freedesktop/FileManager1 ` +
		`--method org.freedesktop.FileManager1.ShowItems ` +
		`["file:///home/me/Shots/a%20b.mp4"] `
	if got != want {
		t.Errorf("args = %q, want %q", got, want)
	}
}

func TestRecordRectFromTheArea(t *testing.T) {
	place := mygo.Rectangle{X: 10, Y: 20, Width: 800, Height: 600}
	got, err := recordRect(options{area: "0,0,200x100"}, nil, image.Rectangle{}, place, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := image.Rect(10, 20, 210, 120); got != want {
		t.Errorf("rect = %v, want %v", got, want)
	}
	if _, err := recordRect(options{area: "nonsense"}, nil, image.Rectangle{}, place, nil); err == nil {
		t.Error("an area of nonsense was taken")
	}
}

func TestOpenedByDesktop(t *testing.T) {
	// What a desktop does to start an app, on the systems that start one:
	// the test runs wherever, and reads the arguments of every system.
	tests := []struct {
		goos   string
		args   []string
		bundle bool
		want   bool
	}{
		{"windows", nil, false, true},
		{"windows", []string{"region"}, false, false},
		{"darwin", nil, true, true},
		{"darwin", []string{"-psn_0_4321"}, true, true},
		{"darwin", []string{"region"}, true, false},
		{"darwin", nil, false, false},
		{"linux", nil, false, false},
		{"linux", []string{"region"}, false, false},
	}
	for _, tt := range tests {
		if got := openedByDesktop(tt.goos, tt.args, tt.bundle); got != tt.want {
			t.Errorf("openedByDesktop(%s, %v, bundle=%v) = %v, want %v", tt.goos, tt.args, tt.bundle, got, tt.want)
		}
	}
}
