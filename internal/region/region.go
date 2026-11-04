// Package region relates the coordinates of the selection window to the
// pixels of a captured screen image.
package region

import (
	"fmt"
	"image"
	"math"
	"strings"
)

// Rect is an axis-aligned rectangle in device-independent pixels.
type Rect struct{ X, Y, W, H float64 }

// Empty reports whether r has no area.
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// FromCorners returns the rectangle spanned by two corners, in any order.
func FromCorners(x0, y0, x1, y1 float64) Rect {
	return Rect{
		X: min(x0, x1),
		Y: min(y0, y1),
		W: math.Abs(x1 - x0),
		H: math.Abs(y1 - y0),
	}
}

// ClampTo returns the part of r inside b.
func (r Rect) ClampTo(b Rect) Rect {
	x0, y0 := max(r.X, b.X), max(r.Y, b.Y)
	x1, y1 := min(r.X+r.W, b.X+b.W), min(r.Y+r.H, b.Y+b.H)
	if x1 <= x0 || y1 <= y0 {
		return Rect{X: x0, Y: y0}
	}
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// Union returns the smallest rectangle holding all of rs.
func Union(rs ...Rect) Rect {
	if len(rs) == 0 {
		return Rect{}
	}
	out := rs[0]
	for _, r := range rs[1:] {
		x0, y0 := min(out.X, r.X), min(out.Y, r.Y)
		x1, y1 := max(out.X+out.W, r.X+r.W), max(out.Y+out.H, r.Y+r.H)
		out = Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
	}
	return out
}

// Mapping places a part of a captured image inside a window.
type Mapping struct {
	// Source is the part of the image the window shows.
	Source image.Rectangle
	// Dst is the window box showing it.
	Dst Rect
}

// Pixels returns the part of the image that r shows, in image pixels.
func (m Mapping) Pixels(r Rect) image.Rectangle {
	if m.Source.Empty() || m.Dst.Empty() {
		return image.Rectangle{}
	}
	r = r.ClampTo(m.Dst)
	if r.Empty() {
		return image.Rectangle{}
	}
	sx := float64(m.Source.Dx()) / m.Dst.W
	sy := float64(m.Source.Dy()) / m.Dst.H
	out := image.Rect(
		int(math.Round(float64(m.Source.Min.X)+(r.X-m.Dst.X)*sx)),
		int(math.Round(float64(m.Source.Min.Y)+(r.Y-m.Dst.Y)*sy)),
		int(math.Round(float64(m.Source.Min.X)+(r.X+r.W-m.Dst.X)*sx)),
		int(math.Round(float64(m.Source.Min.Y)+(r.Y+r.H-m.Dst.Y)*sy)),
	)
	return out.Intersect(m.Source)
}

// Place returns the part of img that shows d, for an image capturing the
// union of the displays.
func Place(union, d Rect, img image.Rectangle) image.Rectangle {
	if union.Empty() || d.Empty() || img.Empty() {
		return img
	}
	sx := float64(img.Dx()) / union.W
	sy := float64(img.Dy()) / union.H
	out := image.Rect(
		int(math.Round(float64(img.Min.X)+(d.X-union.X)*sx)),
		int(math.Round(float64(img.Min.Y)+(d.Y-union.Y)*sy)),
		int(math.Round(float64(img.Min.X)+(d.X+d.W-union.X)*sx)),
		int(math.Round(float64(img.Min.Y)+(d.Y+d.H-union.Y)*sy)),
	)
	return out.Intersect(img)
}

// Area parses "x,y,wxh" into a rectangle in device-independent pixels.
func Area(s string) (Rect, error) {
	var x, y, w, h float64
	if n, err := fmt.Sscanf(strings.TrimSpace(s), "%g,%g,%gx%g", &x, &y, &w, &h); err != nil || n != 4 {
		return Rect{}, fmt.Errorf("kacha: area %q is not x,y,WxH", s)
	}
	if w <= 0 || h <= 0 {
		return Rect{}, fmt.Errorf("kacha: area %q has no size", s)
	}
	return Rect{X: x, Y: y, W: w, H: h}, nil
}
