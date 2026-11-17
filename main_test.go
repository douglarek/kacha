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
