package editor

import (
	"image"
	"math"

	"github.com/egoist/mygo/ui"
)

// Paint draws shapes over the part of the capture that dst shows.
// dips is how many DIPs a pixel of the capture takes, so that a shape is as
// big on screen as it is in the capture.
func Paint(p *ui.Painter, shapes []Shape, mosaic *Mosaic, dips float32, dst ui.Rect) {
	if len(shapes) == 0 || dips <= 0 {
		return
	}
	view := ui.Scaling(dips, dips).Then(ui.Translation(dst.X, dst.Y))
	p.Transform(view, 0, 0, func() {
		for _, s := range shapes {
			paintShape(p, s, mosaic)
		}
	})
}

// paintShape draws one shape in the pixels of the capture.
func paintShape(p *ui.Painter, s Shape, mosaic *Mosaic) {
	switch s.Kind {
	case KindRect:
		r := s.Rect()
		p.Stroke(ui.Rect{
			X: float32(r.Min.X), Y: float32(r.Min.Y),
			W: float32(r.Dx()), H: float32(r.Dy()),
		}, s.Color, 0, s.Width)
	case KindEllipse:
		p.StrokePath(ellipse(s.Rect()), s.Width, s.Color)
	case KindArrow:
		shaft, head := arrow(s)
		p.StrokePath(shaft, s.Width, s.Color)
		p.FillPath(head, s.Color)
	case KindPen:
		p.StrokePath(polyline(s.Pts), s.Width, s.Color)
	case KindText:
		if len(s.Pts) > 0 {
			p.Text(float32(s.Pts[0].X), float32(s.Pts[0].Y), s.Text, s.Width, s.Color)
		}
	case KindMosaic:
		mosaic.Stamp(p, s.Pts, s.Width)
	case KindBadge:
		paintBadge(p, s)
	}
}

// paintBadge draws the number of a badge in a filled circle centred on the
// point it was placed at.
func paintBadge(p *ui.Painter, s Shape) {
	if len(s.Pts) == 0 || s.Text == "" {
		return
	}
	d := badgeDiameter(s.Width)
	cx, cy := float32(s.Pts[0].X), float32(s.Pts[0].Y)
	p.Fill(ui.Rect{X: cx - d/2, Y: cy - d/2, W: d, H: d}, s.Color, d/2)
	size := d * 0.52
	w, h := p.MeasureText(0, ui.Span{Text: s.Text, Size: size})
	p.Text(cx-w/2, cy-h/2, s.Text, size, ui.RGB(255, 255, 255))
}

// badgeDiameter returns the diameter of the circle of a badge, which Width
// holds as the height of a glyph of the text it sits next to.
func badgeDiameter(width float32) float32 {
	return max(width*1.5, 12)
}

// ellipse returns the outline of r.
func ellipse(r image.Rectangle) *ui.Path {
	cx := float32(r.Min.X+r.Max.X) / 2
	cy := float32(r.Min.Y+r.Max.Y) / 2
	rx := float32(r.Dx()) / 2
	ry := float32(r.Dy()) / 2
	const steps = 96
	path := &ui.Path{}
	for i := range steps + 1 {
		a := 2 * math.Pi * float64(i) / steps
		x := cx + rx*float32(math.Cos(a))
		y := cy + ry*float32(math.Sin(a))
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	return path
}

// arrow returns the shaft and the head of an arrow, the head as wide as the
// stroke and as long as four times it.
func arrow(s Shape) (*ui.Path, *ui.Path) {
	from, to := s.Pts[0], s.Pts[len(s.Pts)-1]
	dx, dy := float64(to.X-from.X), float64(to.Y-from.Y)
	length := math.Hypot(dx, dy)
	width := float64(max(s.Width, 1))
	if length == 0 {
		return polyline(s.Pts), &ui.Path{}
	}
	ux, uy := dx/length, dy/length
	head := math.Min(max(width*5, 16), length)
	half := max(width*2.4, 6)
	baseX, baseY := float64(to.X)-ux*head, float64(to.Y)-uy*head

	shaft := (&ui.Path{}).
		MoveTo(float32(from.X), float32(from.Y)).
		LineTo(float32(baseX), float32(baseY))
	headPath := (&ui.Path{}).
		MoveTo(float32(to.X), float32(to.Y)).
		LineTo(float32(baseX-uy*half), float32(baseY+ux*half)).
		LineTo(float32(baseX+uy*half), float32(baseY-ux*half)).
		Close()
	return shaft, headPath
}

// polyline returns pts as a path.
func polyline(pts []image.Point) *ui.Path {
	path := &ui.Path{}
	for i, pt := range pts {
		if i == 0 {
			path.MoveTo(float32(pt.X), float32(pt.Y))
		} else {
			path.LineTo(float32(pt.X), float32(pt.Y))
		}
	}
	return path
}
