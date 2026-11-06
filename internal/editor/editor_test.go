package editor

import (
	"image"
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

// paintInto renders shapes over a blank capture and returns the frame.
func paintInto(t *testing.T, size image.Point, shapes []Shape, dips float32, dst ui.Rect) *image.RGBA {
	t.Helper()
	return ui.Render(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, box ui.Rect) {
			p.Fill(box, ui.RGB(20, 20, 24), 0)
			Paint(p, shapes, nil, dips, dst)
		})
	}, size.X, size.Y, 1)
}

// paintMosaicInto renders shapes over img through its mosaic, and returns
// the frame.
func paintMosaicInto(t *testing.T, img image.Image, shapes []Shape) *image.RGBA {
	t.Helper()
	mosaic := NewMosaic(img)
	return ui.Render(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, box ui.Rect) {
			p.Image(ui.NewBitmap(img), box, ui.FillBox)
			Paint(p, shapes, mosaic, 1, ui.Rect{})
		})
	}, img.Bounds().Dx(), img.Bounds().Dy(), 1)
}

// shades returns a capture whose every pixel has a shade of its own, so that
// a mosaic shows as squares of one shade.
func shades(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x * 2), G: uint8(y * 2), B: 100, A: 255})
		}
	}
	return img
}

func TestPaintPlacesAStrokeWhereItWasDrawn(t *testing.T) {
	shape := Shape{
		Kind:  KindRect,
		Color: ui.RGB(240, 68, 56),
		Width: 4,
		Pts:   []image.Point{{X: 100, Y: 100}, {X: 200, Y: 200}},
	}
	// Two capture pixels a DIP, with the capture's corner at (100, 50).
	frame := paintInto(t, image.Pt(300, 250), []Shape{shape}, 0.5, ui.Rect{X: 100, Y: 50, W: 120, H: 120})

	want := image.Rect(150, 100, 201, 151) // (100+100/2, 50+100/2) to (100+200/2, 50+200/2)
	if !hasColor(frame, want) {
		t.Fatalf("the stroke is not inside %v", want)
	}
	outside := image.Rect(0, 0, 300, 250)
	for y := outside.Min.Y; y < outside.Max.Y; y++ {
		for x := outside.Min.X; x < outside.Max.X; x++ {
			if !image.Pt(x, y).In(want.Inset(-4)) && isRed(frame.At(x, y)) {
				t.Fatalf("the stroke reached %d,%d, outside %v", x, y, want)
			}
		}
	}
}

func TestPaintAtFullSize(t *testing.T) {
	shape := Shape{
		Kind:  KindRect,
		Color: ui.RGB(240, 68, 56),
		Width: 4,
		Pts:   []image.Point{{X: 10, Y: 10}, {X: 40, Y: 30}},
	}
	frame := paintInto(t, image.Pt(64, 48), []Shape{shape}, 1, ui.Rect{})
	// The stroke sits inside the box it outlines.
	for _, pt := range []image.Point{{X: 11, Y: 11}, {X: 38, Y: 11}, {X: 11, Y: 28}, {X: 38, Y: 28}} {
		if !isRed(frame.At(pt.X, pt.Y)) {
			t.Errorf("no stroke at %v", pt)
		}
	}
	if isRed(frame.At(25, 20)) {
		t.Error("the stroke filled its box")
	}
}

func TestNextBadgeNumbersWhatTheBoardHolds(t *testing.T) {
	var b Board
	if got := b.NextBadge(); got != 1 {
		t.Errorf("the first badge is %d, want 1", got)
	}
	badge := func(n string) Shape {
		return Shape{
			Kind:  KindBadge,
			Color: ui.RGB(240, 68, 56),
			Width: 24,
			Pts:   []image.Point{{X: 10, Y: 10}},
			Text:  n,
		}
	}
	b.Add(badge("1"))
	if got := b.NextBadge(); got != 2 {
		t.Errorf("the second badge is %d, want 2", got)
	}
	b.Undo()
	if got := b.NextBadge(); got != 1 {
		t.Errorf("undoing a badge left its number taken: %d", got)
	}
}

func TestPaintBadgeFillsACircleWithItsNumber(t *testing.T) {
	shape := Shape{
		Kind:  KindBadge,
		Color: ui.RGB(240, 68, 56),
		Width: 24,
		Pts:   []image.Point{{X: 32, Y: 32}},
		Text:  "1",
	}
	frame := paintInto(t, image.Pt(64, 64), []Shape{shape}, 1, ui.Rect{})

	if !isRed(frame.At(18, 32)) {
		t.Error("no circle where the badge was placed")
	}
	if !hasWhite(frame, image.Rect(24, 22, 40, 42)) {
		t.Error("no number inside the circle")
	}
	if isRed(frame.At(2, 2)) {
		t.Error("the circle reached the corner")
	}
}

func TestTakeLiftsAShapeOut(t *testing.T) {
	var b Board
	b.Add(Shape{
		Kind:  KindRect,
		Color: ui.RGB(240, 68, 56),
		Width: 4,
		Pts:   []image.Point{{X: 0, Y: 0}, {X: 10, Y: 10}},
	})
	b.Add(Shape{
		Kind:  KindText,
		Color: ui.RGB(240, 68, 56),
		Width: 24,
		Pts:   []image.Point{{X: 5, Y: 5}},
		Text:  "嗨",
	})

	got, ok := b.Take(1)
	if !ok {
		t.Fatal("the text was not taken")
	}
	if got.Text != "嗨" || got.Width != 24 {
		t.Errorf("took %+v, want the text as it was", got)
	}
	if b.Len() != 1 {
		t.Errorf("board holds %d shapes, want the rectangle alone", b.Len())
	}
	if _, ok := b.Take(7); ok {
		t.Error("Take found a shape past the end of the board")
	}
}

func TestPlaceToolbar(t *testing.T) {
	win := ui.Rect{W: 900, H: 600}
	tests := []struct {
		name string
		sel  ui.Rect
		want func(ui.Rect) bool
	}{
		{"under a small selection", ui.Rect{X: 100, Y: 100, W: 200, H: 150}, func(bar ui.Rect) bool {
			return bar.Y >= 250 && bar.X+bar.W <= win.W && bar.X >= 0
		}},
		{"over a selection that reaches the bottom", ui.Rect{X: 100, Y: 100, W: 200, H: 440}, func(bar ui.Rect) bool {
			return bar.Y+bar.H <= 100
		}},
		{"along the bottom when neither fits", ui.Rect{X: 0, Y: 0, W: 900, H: 600}, func(bar ui.Rect) bool {
			return bar.Y+bar.H <= win.H && bar.Y > 0
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := Layout(tt.sel, win, State{Tool: KindRect, Color: Colors()[0], Width: Widths()[0]})
			if !tt.want(bar.Rect) {
				t.Errorf("bar at %+v does not fit the case", bar.Rect)
			}
			for _, button := range bar.Buttons {
				if b := button.Rect; b.X < 0 || b.Y < 0 || b.X+b.W > win.W || b.Y+b.H > win.H {
					t.Errorf("%s at %+v leaves the window", buttonName(button), b)
				}
			}
		})
	}
}

func TestBarButtons(t *testing.T) {
	bar := Layout(ui.Rect{X: 100, Y: 100, W: 200, H: 150}, ui.Rect{W: 600, H: 400}, State{})
	if len(bar.Buttons) != len(Tools())+len(Colors())+len(Widths())+5 {
		t.Errorf("bar holds %d buttons", len(bar.Buttons))
	}
	if _, ok := bar.Button(CommandUndo); !ok {
		t.Error("no undo button")
	}
	if _, ok := bar.Tool(KindMosaic); !ok {
		t.Error("no mosaic tool")
	}
	if _, ok := bar.Tool(KindCrop); !ok {
		t.Error("no crop tool")
	}
	button, _ := bar.Tool(KindArrow)
	if hit, ok := bar.Hit(ui.Point{X: button.Rect.X + 2, Y: button.Rect.Y + 2}); !ok || hit.Rect != button.Rect {
		t.Errorf("hit = %+v, want the arrow button", hit.Rect)
	}
}

func TestEveryButtonHasATip(t *testing.T) {
	bar := Layout(ui.Rect{X: 100, Y: 100, W: 200, H: 150}, ui.Rect{W: 600, H: 400}, State{})
	for _, button := range bar.Buttons {
		if button.Tip == "" {
			t.Errorf("%s has no tip", buttonName(button))
		}
	}
}

func TestPixelate(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 2 {
			src.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 255})
			src.Set(x+2, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}
	got := Pixelate(src, 2)
	if got.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("bounds = %v, want 2x2", got.Bounds())
	}
	if c := got.RGBAAt(0, 0); c.R != 0 {
		t.Errorf("left block = %v, want black", c)
	}
	if c := got.RGBAAt(1, 0); c.R != 200 {
		t.Errorf("right block = %v, want light grey", c)
	}
}

func TestMosaicCoversTheBoxItWasDraggedOver(t *testing.T) {
	src := shades(96, 96)
	shape := Shape{
		Kind: KindMosaic,
		Pts:  []image.Point{{X: 30, Y: 40}, {X: 70, Y: 64}},
	}
	frame := paintMosaicInto(t, src, []Shape{shape})

	// The squares grow past the box to the grid of the whole capture.
	if !flatSquares(frame, image.Rect(24, 36, 72, 72)) {
		t.Error("the box kept the shades of the capture")
	}
	for _, pt := range []image.Point{{X: 18, Y: 30}, {X: 80, Y: 80}} {
		if frame.At(pt.X, pt.Y) != src.At(pt.X, pt.Y) {
			t.Errorf("the mosaic reached %v, outside the box", pt)
		}
	}
}

func TestMosaicAreaIsWhatTheShapeCovers(t *testing.T) {
	src := shades(96, 96)
	mosaic := NewMosaic(src)

	// A box grows to the squares it touches, as the squares it draws do.
	box := mosaic.Area(Shape{Kind: KindMosaic, Pts: []image.Point{{X: 30, Y: 40}, {X: 70, Y: 64}}})
	if want := image.Rect(24, 36, 72, 72); box != want {
		t.Errorf("the box covers %v, want %v", box, want)
	}
	// A stroke covers whatever its stamps touch, along the whole path.
	stroke := mosaic.Area(Shape{
		Kind:  KindMosaicBrush,
		Width: 8,
		Pts:   []image.Point{{X: 20, Y: 20}, {X: 60, Y: 20}},
	})
	if want := image.Rect(12, 12, 72, 36); stroke != want {
		t.Errorf("the stroke covers %v, want %v", stroke, want)
	}
	if empty := mosaic.Area(Shape{Kind: KindMosaicBrush}); !empty.Empty() {
		t.Errorf("a brush with no points covers %v", empty)
	}
}

func TestMosaicBrushFollowsThePath(t *testing.T) {
	src := shades(96, 96)
	shape := Shape{
		Kind:  KindMosaicBrush,
		Width: 8,
		Pts:   []image.Point{{X: 20, Y: 20}, {X: 40, Y: 20}, {X: 60, Y: 20}},
	}
	frame := paintMosaicInto(t, src, []Shape{shape})

	if !flatSquares(frame, image.Rect(12, 12, 72, 36)) {
		t.Error("the path kept the shades of the capture")
	}
	if frame.At(20, 60) != src.At(20, 60) {
		t.Error("the brush painted away from its path")
	}
}

func TestLayoutPicker(t *testing.T) {
	for _, win := range []ui.Rect{{W: 1000, H: 640}, {W: 520, H: 420}} {
		bar := Layout(ui.Rect{X: 100, Y: 100, W: 400, H: 300}, win, State{Tool: KindEmoji, Width: Widths()[1]})
		picker := LayoutPicker(bar, win, Emojis()[3])

		if len(picker.Buttons) != len(Emojis()) {
			t.Fatalf("the panel holds %d emoji, want %d", len(picker.Buttons), len(Emojis()))
		}
		if p := picker.Rect; p.X < 0 || p.Y < 0 || p.X+p.W > win.W || p.Y+p.H > win.H {
			t.Errorf("in a %g×%g window, the panel %+v leaves the window", win.W, win.H, p)
		}
		if picker.Rect.Y+picker.Rect.H > bar.Rect.Y && win.W > 600 {
			t.Errorf("the panel %+v covers the toolbar %+v", picker.Rect, bar.Rect)
		}
		for i, button := range picker.Buttons {
			if button.Text != Emojis()[i] {
				t.Errorf("cell %d shows %q, want %q", i, button.Text, Emojis()[i])
			}
			if b := button.Rect; b.X < picker.Rect.X || b.Y < picker.Rect.Y || b.X+b.W > picker.Rect.X+picker.Rect.W || b.Y+b.H > picker.Rect.Y+picker.Rect.H {
				t.Errorf("cell %d at %+v leaves the panel", i, b)
			}
			if active := button.Action.Emoji == Emojis()[3]; button.Active != active {
				t.Errorf("cell %d is lit: %v", i, button.Active)
			}
		}
	}
}

func TestPaintEmojiDrawsItsGlyphOverThePoint(t *testing.T) {
	shape := Shape{
		Kind:  KindEmoji,
		Width: 32,
		Pts:   []image.Point{{X: 40, Y: 40}},
		Text:  "🎉",
	}
	frame := paintInto(t, image.Pt(80, 80), []Shape{shape}, 1, ui.Rect{})

	if !hasColorOf(frame, image.Rect(20, 20, 60, 60)) {
		t.Error("no glyph where the emoji was placed")
	}
	if hasColorOf(frame, image.Rect(0, 0, 12, 12)) {
		t.Error("the glyph reached the corner, away from its point")
	}
}

// hasColorOf reports whether any pixel inside r has colors of its own, as a
// color emoji's have.
func hasColorOf(img image.Image, r image.Rectangle) bool {
	for y := max(r.Min.Y, 0); y < min(r.Max.Y, img.Bounds().Dy()); y++ {
		for x := max(r.Min.X, 0); x < min(r.Max.X, img.Bounds().Dx()); x++ {
			red, green, blue, _ := img.At(x, y).RGBA()
			most := max(red, green, blue)
			least := min(red, green, blue)
			if most-least > 0x2000 {
				return true
			}
		}
	}
	return false
}

// flatSquares reports whether every Block×Block square of img inside r is one
// shade, as the squares of a mosaic are.
func flatSquares(img image.Image, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if img.At(x, y) != img.At(x-x%Block, y-y%Block) {
				return false
			}
		}
	}
	return true
}

// hasColor reports whether any pixel inside r has the toolbar's red.
func hasColor(img image.Image, r image.Rectangle) bool {
	for y := max(r.Min.Y, 0); y < min(r.Max.Y, img.Bounds().Dy()); y++ {
		for x := max(r.Min.X, 0); x < min(r.Max.X, img.Bounds().Dx()); x++ {
			if isRed(img.At(x, y)) {
				return true
			}
		}
	}
	return false
}

// isRed reports whether c is the red shapes are drawn in.
func isRed(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r > 0xd000 && g < 0x6000 && b < 0x6000
}

// hasWhite reports whether any pixel inside r is white, as the number of a
// badge is.
func hasWhite(img image.Image, r image.Rectangle) bool {
	for y := max(r.Min.Y, 0); y < min(r.Max.Y, img.Bounds().Dy()); y++ {
		for x := max(r.Min.X, 0); x < min(r.Max.X, img.Bounds().Dx()); x++ {
			c := img.At(x, y)
			rr, gg, bb, _ := c.RGBA()
			if rr > 0xe000 && gg > 0xe000 && bb > 0xe000 {
				return true
			}
		}
	}
	return false
}

// buttonName describes a button for a failed test.
func buttonName(b Button) string {
	for _, tool := range Tools() {
		if b.Action.Kind == ActionTool && b.Action.Tool == tool.Kind {
			return tool.Name
		}
	}
	return "button"
}
