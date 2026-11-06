// Package editor holds the annotations a capture carries, draws them in the
// selection window, and renders them over the capture at its own size.
package editor

import (
	"image"

	"github.com/egoist/mygo/ui"
)

// Kind is what a shape draws.
type Kind uint8

const (
	KindRect Kind = iota
	KindEllipse
	KindArrow
	KindPen
	KindText
	// KindMosaic covers the box its corners span with squares.
	KindMosaic
	// KindMosaicBrush follows Pts as a pen does, covering the squares of the
	// mosaic under its path.
	KindMosaicBrush
	// KindBadge numbers what the other shapes point at: a filled circle
	// with the number in Text.
	KindBadge
	// KindEmoji shows Text, an emoji, centred on the one point of Pts.
	KindEmoji
	// KindCrop is the toolbar's crop: a drag with it picks the part of the
	// capture to keep, so no shape is ever drawn with it.
	KindCrop
)

// Shape is one annotation, in the pixels of the capture.
type Shape struct {
	Kind  Kind
	Color ui.Color
	// Width is the stroke width, the radius a mosaic brush stamps with, or
	// for a text or an emoji the size of its glyph, in pixels of the
	// capture.
	Width float32
	// Pts are the corners of a rect, ellipse, arrow or mosaic, the path of a
	// pen or mosaic brush, the top left corner of a text, and the point an
	// emoji sits on.
	Pts []image.Point
	// Text is what a text shape shows, and the emoji an emoji shape shows.
	Text string
}

// Rect returns the box two corners span.
func (s Shape) Rect() image.Rectangle {
	if len(s.Pts) < 2 {
		if len(s.Pts) == 1 {
			return image.Rectangle{s.Pts[0], s.Pts[0]}
		}
		return image.Rectangle{}
	}
	return image.Rectangle{
		Min: image.Point{X: min(s.Pts[0].X, s.Pts[1].X), Y: min(s.Pts[0].Y, s.Pts[1].Y)},
		Max: image.Point{X: max(s.Pts[0].X, s.Pts[1].X), Y: max(s.Pts[0].Y, s.Pts[1].Y)},
	}
}

// Empty reports whether the shape draws nothing.
func (s Shape) Empty() bool {
	switch s.Kind {
	case KindText:
		return s.Text == ""
	case KindBadge, KindEmoji:
		return len(s.Pts) == 0 || s.Text == ""
	case KindPen, KindMosaicBrush:
		return len(s.Pts) < 2
	default:
		r := s.Rect()
		return r.Dx() < 2 || r.Dy() < 2
	}
}

// Board is the annotations of a capture with the history of Undo.
type Board struct {
	shapes []Shape
	undone []Shape
}

// Add puts s on top of the others.
func (b *Board) Add(s Shape) {
	if s.Empty() {
		return
	}
	b.shapes = append(b.shapes, s)
	b.undone = b.undone[:0]
}

// Shift moves every shape by dx and dy pixels, for a capture whose top
// left corner moved.
func (b *Board) Shift(dx, dy int) {
	if dx == 0 && dy == 0 {
		return
	}
	for i := range b.shapes {
		for j := range b.shapes[i].Pts {
			b.shapes[i].Pts[j].X += dx
			b.shapes[i].Pts[j].Y += dy
		}
	}
}

// Replace puts shape where the shape at index was, which is how a text the
// pointer drags moves.
func (b *Board) Replace(index int, shape Shape) bool {
	if index < 0 || index >= len(b.shapes) {
		return false
	}
	b.shapes[index] = shape
	b.undone = b.undone[:0]
	return true
}

// Take removes the shape at index and returns it, which is how a text is
// typed again.
func (b *Board) Take(index int) (Shape, bool) {
	if index < 0 || index >= len(b.shapes) {
		return Shape{}, false
	}
	shape := b.shapes[index]
	b.shapes = append(b.shapes[:index], b.shapes[index+1:]...)
	b.undone = b.undone[:0]
	return shape, true
}

// Undo takes off the last shape, which Redo puts back.
func (b *Board) Undo() bool {
	if len(b.shapes) == 0 {
		return false
	}
	b.undone = append(b.undone, b.shapes[len(b.shapes)-1])
	b.shapes = b.shapes[:len(b.shapes)-1]
	return true
}

// Redo puts back the shape Undo took off.
func (b *Board) Redo() bool {
	if len(b.undone) == 0 {
		return false
	}
	b.shapes = append(b.shapes, b.undone[len(b.undone)-1])
	b.undone = b.undone[:len(b.undone)-1]
	return true
}

// CanRedo reports whether Undo took off a shape Redo puts back.
func (b *Board) CanRedo() bool { return len(b.undone) > 0 }

// NextBadge returns the number a badge placed now shows: one past the badges
// the board holds, so that undoing one frees its number again.
func (b *Board) NextBadge() int {
	n := 0
	for _, s := range b.shapes {
		if s.Kind == KindBadge {
			n++
		}
	}
	return n + 1
}

// Len returns how many shapes the board holds.
func (b *Board) Len() int { return len(b.shapes) }

// Shapes returns the shapes from the first to the last.
func (b *Board) Shapes() []Shape { return b.shapes }

// Extent returns the box a text or an emoji covers, in the pixels of the
// capture, which the pointer uses to tell which one it lands on. A text's
// width is estimated from the glyphs, a wide one taking its height and a
// narrow one a little over half of it; an emoji is a square of its size,
// whatever its glyphs are made of.
func Extent(s Shape) (float32, float32) {
	if s.Kind == KindEmoji {
		return s.Width, s.Width * 1.15
	}
	width := float32(0)
	for _, r := range s.Text {
		if r >= 0x2e80 {
			width += s.Width
			continue
		}
		width += s.Width * 0.55
	}
	return width, s.Width * 1.15
}

// Bounds returns the box a shape covers, in the pixels of the capture.
func (s Shape) Bounds() image.Rectangle {
	switch s.Kind {
	case KindText, KindEmoji, KindBadge:
		if len(s.Pts) == 0 {
			return image.Rectangle{}
		}
		w, h := Extent(s)
		at := s.Pts[0]
		if s.Kind == KindText {
			return image.Rect(at.X, at.Y, at.X+int(w), at.Y+int(h))
		}
		// An emoji and a badge sit centred on their point.
		return image.Rect(at.X-int(w/2), at.Y-int(h/2), at.X+int(w/2), at.Y+int(h/2))
	case KindPen, KindMosaicBrush:
		if len(s.Pts) == 0 {
			return image.Rectangle{}
		}
		box := image.Rectangle{Min: s.Pts[0], Max: s.Pts[0]}
		for _, pt := range s.Pts[1:] {
			box = box.Union(image.Rectangle{Min: pt, Max: pt})
		}
		pad := int(s.Width / 2)
		return box.Inset(-pad)
	default:
		return s.Rect()
	}
}

// Keep drops the shapes the capture no longer shows, which a crop leaves
// outside what it kept: a shape that is not in the capture is in the dark
// around the selection, where nothing should be.
func (b *Board) Keep(bounds image.Rectangle) {
	kept := b.shapes[:0]
	for _, shape := range b.shapes {
		if shape.Bounds().Overlaps(bounds) {
			kept = append(kept, shape)
		}
	}
	b.shapes = kept
	b.undone = b.undone[:0]
}
