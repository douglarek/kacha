package main

import (
	"image"
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
