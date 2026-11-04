package overlay

import (
	"image"
	"image/color"
	"math"
)

// windowRadius is the corner radius, in DIPs, of the windows macOS draws.
const windowRadius = 12

// roundCrop clears what lies outside the rounded corners of img, so that a
// window keeps the corners the system gave it instead of carrying the
// pixels of what was behind them.
func roundCrop(img *image.RGBA, radius float64) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	r := math.Min(radius, float64(min(w, h))/2)
	if r < 1 {
		return
	}
	touch := func(x, y int, cx, cy float64) {
		d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
		cover := math.Min(math.Max(r-d+0.5, 0), 1)
		if cover >= 1 {
			return
		}
		c := img.RGBAAt(x, y)
		alpha := float64(c.A) * cover
		img.SetRGBA(x, y, color.RGBA{R: c.R, G: c.G, B: c.B, A: uint8(alpha)})
	}
	for y := range int(math.Ceil(r)) {
		for x := range int(math.Ceil(r)) {
			touch(x, y, r, r)
			touch(w-1-x, y, float64(w)-r, r)
			touch(x, h-1-y, r, float64(h)-r)
			touch(w-1-x, h-1-y, float64(w)-r, float64(h)-r)
		}
	}
}
