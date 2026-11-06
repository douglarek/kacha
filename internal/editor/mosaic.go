package editor

import (
	"image"
	"image/color"
	"math"

	"github.com/egoist/mygo/ui"
)

// Block is the side, in capture pixels, of the squares a Mosaic paints.
const Block = 12

// Mosaic is the pixelated copy of a capture that the mosaic tool stamps.
type Mosaic struct {
	Bitmap *ui.Bitmap
	// Block is the side of one square in pixels of the capture.
	Block float32
}

// NewMosaic returns the copy of img the mosaic tool paints, made of
// Block×Block squares.
func NewMosaic(img image.Image) *Mosaic {
	return &Mosaic{Bitmap: ui.NewBitmap(Pixelate(img, Block)).Pixelated(), Block: Block}
}

// Stamp draws the pixels of the mosaic under pts, a brush of radius r.
// Squares snap to the grid of the whole capture, so stamps that overlap
// line up.
func (m *Mosaic) Stamp(p *ui.Painter, pts []image.Point, r float32) {
	if m == nil || m.Bitmap == nil || m.Block <= 0 {
		return
	}
	r = max(r, m.Block/2)
	var last image.Rectangle
	for i, pt := range pts {
		src := m.rectAt(pt, r)
		if i > 0 && src == last {
			continue
		}
		last = src
		m.draw(p, src)
	}
}

// Cover draws the pixels of the mosaic inside r, a box in the pixels of the
// capture, grown to the squares it touches, as a stamp is.
func (m *Mosaic) Cover(p *ui.Painter, r image.Rectangle) {
	if m == nil || m.Bitmap == nil || m.Block <= 0 || r.Empty() {
		return
	}
	m.draw(p, m.grid(r))
}

// Area returns the box of the capture s covers with squares, which is where a
// frame around it goes: what a heavy hand covers is hard to tell from what it
// hides.
func (m *Mosaic) Area(s Shape) image.Rectangle {
	if m == nil || m.Bitmap == nil || m.Block <= 0 {
		return image.Rectangle{}
	}
	cover := image.Rectangle{}
	if s.Kind == KindMosaic {
		cover = m.grid(s.Rect())
	} else {
		for i, pt := range s.Pts {
			r := m.rectAt(pt, max(s.Width, m.Block/2))
			if i == 0 {
				cover = r
				continue
			}
			cover = cover.Union(r)
		}
	}
	side := int(m.Block)
	w, h := m.Bitmap.Size()
	return cover.Intersect(image.Rect(0, 0, w*side, h*side))
}

// draw shows the squares of the mosaic inside src, which it cuts to the
// capture.
func (m *Mosaic) draw(p *ui.Painter, src image.Rectangle) {
	side := int(m.Block)
	w, h := m.Bitmap.Size()
	src = src.Intersect(image.Rect(0, 0, w*side, h*side))
	if src.Empty() {
		return
	}
	p.ImageRegion(m.Bitmap, ui.Rect{
		X: float32(src.Min.X) / m.Block, Y: float32(src.Min.Y) / m.Block,
		W: float32(src.Dx()) / m.Block, H: float32(src.Dy()) / m.Block,
	}, ui.Rect{
		X: float32(src.Min.X), Y: float32(src.Min.Y),
		W: float32(src.Dx()), H: float32(src.Dy()),
	}, ui.ImageOptions{})
}

// grid returns r grown to the squares of the mosaic. Every shape snaps to
// the same squares, so that the ones next to each other line up.
func (m *Mosaic) grid(r image.Rectangle) image.Rectangle {
	b, side := float64(m.Block), int(m.Block)
	return image.Rect(
		int(math.Floor(float64(r.Min.X)/b))*side,
		int(math.Floor(float64(r.Min.Y)/b))*side,
		int(math.Ceil(float64(r.Max.X)/b))*side,
		int(math.Ceil(float64(r.Max.Y)/b))*side,
	)
}

// rectAt returns the squares the brush at pt covers, on the grid.
func (m *Mosaic) rectAt(pt image.Point, r float32) image.Rectangle {
	return m.grid(image.Rect(
		int(float64(pt.X)-float64(r)), int(float64(pt.Y)-float64(r)),
		int(float64(pt.X)+float64(r)), int(float64(pt.Y)+float64(r)),
	))
}

// Pixelate returns img as block×block squares of its average color.
func Pixelate(img image.Image, block int) *image.RGBA {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	out := image.NewRGBA(image.Rect(0, 0, max(w/block, 1), max(h/block, 1)))
	for by := range out.Bounds().Dy() {
		for bx := range out.Bounds().Dx() {
			var r, g, b, a, n uint64
			for y := by * block; y < (by+1)*block && y < h; y++ {
				for x := bx * block; x < (bx+1)*block && x < w; x++ {
					pr, pg, pb, pa := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
					r += uint64(pr >> 8)
					g += uint64(pg >> 8)
					b += uint64(pb >> 8)
					a += uint64(pa >> 8)
					n++
				}
			}
			if n == 0 {
				continue
			}
			out.Set(bx, by, color.RGBA{
				R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: uint8(a / n),
			})
		}
	}
	return out
}
