package capture

import (
	"image"
	"image/color"
	"testing"
)

func TestCropKeepsTheArea(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 10))
	src.Set(5, 5, color.RGBA{R: 255, A: 255})
	got := Crop(src, image.Rect(5, 5, 8, 8))
	if got.Bounds() != image.Rect(0, 0, 3, 3) {
		t.Fatalf("crop bounds = %v, want a 3x3 image", got.Bounds())
	}
	if r, _, _, _ := got.At(0, 0).RGBA(); r == 0 {
		t.Error("the cropped pixel lost its color")
	}
}

func TestCropClampsToTheImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 10))
	got := Crop(src, image.Rect(5, 5, 50, 50))
	if want := image.Rect(0, 0, 5, 5); got.Bounds() != want {
		t.Errorf("crop bounds = %v, want %v", got.Bounds(), want)
	}
}
