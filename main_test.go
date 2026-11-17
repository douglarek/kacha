package main

import (
	"image"
	"os"
	"runtime"
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

func TestOpenedAlone(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("a desktop opens an app on macOS and Windows alone")
	}
	args := os.Args
	defer func() { os.Args = args }()

	os.Args = []string{"kacha"}
	t.Setenv("__CFBundleIdentifier", "vibe.kacha")
	if !openedAlone() {
		t.Error("an app opened with no arguments is not alone")
	}

	// The desktop numbers the process as it opens an app, which the app
	// takes as no argument at all.
	os.Args = []string{"kacha", "-psn_0_4321"}
	if !openedAlone() {
		t.Error("the process number of the desktop counted as an argument")
	}

	os.Args = []string{"kacha", "region"}
	if openedAlone() {
		t.Error("a command counted as no argument")
	}

	// A capture asked for from a shell takes one.
	os.Args = []string{"kacha"}
	t.Setenv("__CFBundleIdentifier", "")
	if openedAlone() {
		t.Error("a copy run from a shell counted as opened by the desktop")
	}
}
