package overlay

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/douglarek/kacha/internal/editor"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// TestRecordDemo records the overlay, the editor and its toolbar for the
// README, into KACHA_DEMO_DIR, one PNG a frame.
//
//	KACHA_DEMO_DIR=/tmp/frames go test ./internal/overlay -run TestRecordDemo
func TestRecordDemo(t *testing.T) {
	dir := os.Getenv("KACHA_DEMO_DIR")
	if dir == "" {
		t.Skip("KACHA_DEMO_DIR is not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := desktop(2000, 1280)
	window := image.Rect(220, 140, 1780, 1140)
	s := newSelector(Options{
		Image:   src,
		Source:  src.Bounds(),
		Place:   mygo.Rectangle{Width: 1000, Height: 640},
		Windows: []Window{{Rect: window, App: "Melo", Round: true}},
	}, nil)
	tt := ui.NewTester(s.view, 1000, 640)

	frame := 0
	shot := func(times int) {
		for range times {
			img := tt.Image()
			file, err := os.Create(filepath.Join(dir, fmt.Sprintf("%03d.png", frame)))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(file, img); err != nil {
				file.Close()
				t.Fatal(err)
			}
			file.Close()
			frame++
		}
	}
	// move walks the pointer to x, y in steps, shooting each one.
	move := func(x, y float32, steps int) {
		fromX, fromY := lastPointer(t, s)
		for i := 1; i <= steps; i++ {
			f := float32(i) / float32(steps)
			tt.Move(fromX+(x-fromX)*f, fromY+(y-fromY)*f)
			shot(1)
		}
	}
	click := func(x, y float32) {
		tt.Move(x, y)
		shot(1)
		tt.Press(x, y)
		shot(1)
		tt.Release(x, y)
		shot(2)
	}
	tool := func(kind editor.Kind) {
		button, ok := s.bar.Tool(kind)
		if !ok {
			t.Fatalf("no %v in the toolbar", kind)
		}
		click(button.Rect.X+button.Rect.W/2, button.Rect.Y+button.Rect.H/2)
	}

	shot(4)
	move(500, 280, 14)
	shot(4)
	click(500, 280)
	shot(3)

	tool(editor.KindRect)
	move(320, 200, 8)
	tt.Press(320, 200)
	shot(1)
	move(500, 330, 12)
	tt.Release(500, 330)
	shot(3)

	tool(editor.KindArrow)
	move(330, 460, 8)
	tt.Press(330, 460)
	shot(1)
	move(620, 390, 10)
	tt.Release(620, 390)
	shot(3)

	tool(editor.KindBadge)
	click(700, 210)
	shot(3)

	tool(editor.KindMosaic)
	// Across the edge of the album art and the rectangle, where the squares
	// of the mosaic show.
	move(420, 150, 8)
	tt.Press(420, 150)
	shot(1)
	move(540, 260, 8)
	tt.Release(540, 260)
	shot(4)

	tool(editor.KindEmoji)
	if len(s.picker.Buttons) == 0 {
		t.Fatal("the emoji tool showed no panel")
	}
	// A cell puts its emoji in the middle of the selection, where the drag
	// that follows moves it.
	cell := s.picker.Buttons[2]
	click(cell.Rect.X+cell.Rect.W/2, cell.Rect.Y+cell.Rect.H/2)
	shot(2)
	// The selection is the window the demo clicked first, whose middle is
	// where the emoji lands.
	move(500, 320, 5)
	tt.Press(500, 320)
	shot(1)
	move(650, 430, 5)
	tt.Release(650, 430)
	shot(3)

	button, ok := s.bar.Button(editor.CommandCopy)
	if !ok {
		t.Fatal("no copy button")
	}
	move(button.Rect.X+button.Rect.W/2, button.Rect.Y+button.Rect.H/2, 10)
	shot(4)
	tt.ClickAt(button.Rect.X+button.Rect.W/2, button.Rect.Y+button.Rect.H/2)
	shot(2)
}

// lastPointer returns where the pointer is, in the window's DIPs.
func lastPointer(t *testing.T, s *selector) (float32, float32) {
	t.Helper()
	return s.at[0], s.at[1]
}

// desktop draws a stand-in for a screen: an app with a sidebar, a list and a
// player.
func desktop(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(28 + x*20/w), G: uint8(32 + y*24/h), B: 52, A: 255})
		}
	}
	const (
		x0, y0 = 220, 140
		x1, y1 = 1780, 1140
	)
	draw.Draw(img, image.Rect(x0+8, y0+8, x1, y1), image.NewUniform(color.RGBA{0, 0, 0, 70}), image.Point{}, draw.Over)
	draw.Draw(img, image.Rect(x0, y0, x1, y1), image.NewUniform(color.RGBA{R: 246, G: 246, B: 248, A: 255}), image.Point{}, draw.Src)
	// the title bar
	draw.Draw(img, image.Rect(x0, y0, x1, y0+60), image.NewUniform(color.RGBA{R: 232, G: 233, B: 238, A: 255}), image.Point{}, draw.Src)
	for i, c := range []color.RGBA{{242, 107, 94, 255}, {242, 201, 76, 255}, {90, 195, 125, 255}} {
		draw.Draw(img, image.Rect(x0+28+i*44, y0+24, x0+50+i*44, y0+46), image.NewUniform(c), image.Point{}, draw.Src)
	}
	// the sidebar
	draw.Draw(img, image.Rect(x0, y0+60, x0+320, y1), image.NewUniform(color.RGBA{R: 238, G: 239, B: 244, A: 255}), image.Point{}, draw.Src)
	for i := range 7 {
		y := y0 + 110 + i*74
		draw.Draw(img, image.Rect(x0+40, y, x0+240, y+18), image.NewUniform(color.RGBA{200, 202, 212, 255}), image.Point{}, draw.Src)
	}
	// the list, with an album of it
	art := image.Rect(x0+380, y0+120, x0+700, y0+440)
	for y := art.Min.Y; y < art.Max.Y; y++ {
		for x := art.Min.X; x < art.Max.X; x++ {
			f := float64(y-art.Min.Y) / float64(art.Dy())
			img.Set(x, y, color.RGBA{R: uint8(200 - 120*f), G: uint8(90 + 40*f), B: 110, A: 255})
		}
	}
	for i := range 6 {
		y := y0 + 500 + i*90
		draw.Draw(img, image.Rect(x0+380, y, x0+1080, y+22), image.NewUniform(color.RGBA{206, 208, 218, 255}), image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(x0+380, y+34, x0+700, y+52), image.NewUniform(color.RGBA{222, 224, 232, 255}), image.Point{}, draw.Src)
	}
	// the player
	draw.Draw(img, image.Rect(x0, y1-120, x1, y1), image.NewUniform(color.RGBA{R: 252, G: 252, B: 253, A: 255}), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(x0+380, y1-70, x1-80, y1-58), image.NewUniform(color.RGBA{214, 216, 224, 255}), image.Point{}, draw.Src)
	return img
}
