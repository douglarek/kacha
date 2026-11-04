package region

import (
	"image"
	"testing"
)

func TestFromCorners(t *testing.T) {
	tests := []struct {
		name           string
		x0, y0, x1, y1 float64
		x, y, w, h     float64
	}{
		{"dragged right and down", 10, 20, 60, 80, 10, 20, 50, 60},
		{"dragged up and left", 60, 80, 10, 20, 10, 20, 50, 60},
		{"dragged right and up", 10, 80, 60, 20, 10, 20, 50, 60},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromCorners(tt.x0, tt.y0, tt.x1, tt.y1)
			if got != (Rect{X: tt.x, Y: tt.y, W: tt.w, H: tt.h}) {
				t.Errorf("FromCorners = %+v, want %v,%v %vx%v", got, tt.x, tt.y, tt.w, tt.h)
			}
		})
	}
}

func TestClampTo(t *testing.T) {
	window := Rect{X: 0, Y: 0, W: 100, H: 100}
	got := (Rect{X: -20, Y: 80, W: 60, H: 60}).ClampTo(window)
	want := Rect{X: 0, Y: 80, W: 40, H: 20}
	if got != want {
		t.Errorf("ClampTo = %+v, want %+v", got, want)
	}
	if !(Rect{X: 200, Y: 200, W: 10, H: 10}).ClampTo(window).Empty() {
		t.Error("a rectangle outside the window keeps an area")
	}
}

func TestUnion(t *testing.T) {
	got := Union(
		Rect{X: 0, Y: 0, W: 1920, H: 1080},
		Rect{X: 1920, Y: 200, W: 1280, H: 1024},
	)
	want := Rect{X: 0, Y: 0, W: 3200, H: 1224}
	if got != want {
		t.Errorf("Union = %+v, want %+v", got, want)
	}
}

func TestPixelsScales(t *testing.T) {
	m := Mapping{
		Source: image.Rect(0, 0, 2560, 1600),
		Dst:    Rect{X: 0, Y: 0, W: 1280, H: 800},
	}
	got := m.Pixels(Rect{X: 100, Y: 50, W: 200, H: 100})
	want := image.Rect(200, 100, 600, 300)
	if got != want {
		t.Errorf("Pixels = %v, want %v", got, want)
	}
}

func TestPixelsStaysInsideTheSource(t *testing.T) {
	m := Mapping{
		Source: image.Rect(100, 100, 300, 300),
		Dst:    Rect{X: 0, Y: 0, W: 200, H: 200},
	}
	got := m.Pixels(Rect{X: -50, Y: 150, W: 400, H: 100})
	want := image.Rect(100, 250, 300, 300)
	if got != want {
		t.Errorf("Pixels = %v, want %v", got, want)
	}
}

func TestPlaceFindsTheDisplay(t *testing.T) {
	img := image.Rect(0, 0, 3200, 1080)
	union := Rect{X: 0, Y: 0, W: 3200, H: 1080}
	second := Rect{X: 1920, Y: 0, W: 1280, H: 1080}
	if got, want := Place(union, second, img), image.Rect(1920, 0, 3200, 1080); got != want {
		t.Errorf("Place = %v, want %v", got, want)
	}
	scaled := Rect{X: 0, Y: 0, W: 1280, H: 540}
	if got, want := Place(union, scaled, img), image.Rect(0, 0, 1280, 540); got != want {
		t.Errorf("Place with a scale factor = %v, want %v", got, want)
	}
}

func TestArea(t *testing.T) {
	got, err := Area("10,20,300x200")
	if err != nil {
		t.Fatal(err)
	}
	if want := (Rect{X: 10, Y: 20, W: 300, H: 200}); got != want {
		t.Errorf("Area = %+v, want %+v", got, want)
	}
	for _, bad := range []string{"", "10,20", "10,20,300", "10,20,0x10", "ten,20,30x40"} {
		if _, err := Area(bad); err == nil {
			t.Errorf("Area(%q) accepted", bad)
		}
	}
}
