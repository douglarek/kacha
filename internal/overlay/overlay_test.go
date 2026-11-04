package overlay

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/douglarek/kacha/internal/editor"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// screen draws a stand-in for a captured desktop: a gradient with blocks.
func screen(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{
				R: uint8(40 + x*80/w),
				G: uint8(70 + y*120/h),
				B: 160,
				A: 255,
			})
		}
	}
	draw.Draw(img, image.Rect(80, 80, 380, 240), image.NewUniform(color.RGBA{R: 240, G: 240, B: 245, A: 255}), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(440, 120, 700, 380), image.NewUniform(color.RGBA{R: 250, G: 214, B: 170, A: 255}), image.Point{}, draw.Src)
	// Lines stand in for the text a mosaic hides.
	for y := 240; y < 470; y += 16 {
		draw.Draw(img, image.Rect(240, y, 560, y+7), image.NewUniform(color.RGBA{R: 30, G: 40, B: 60, A: 255}), image.Point{}, draw.Src)
	}
	return img
}

// open returns a selector over a fake desktop, and its tester.
func open(t *testing.T, done func(image.Image, Choice) error) (*selector, *ui.Tester) {
	t.Helper()
	src := screen(1200, 800)
	s := newSelector(Options{
		Image:  src,
		Source: src.Bounds(),
		Place:  mygo.Rectangle{Width: 600, Height: 400},
	}, done)
	return s, ui.NewTester(s.view, 600, 400)
}

// selectArea drags the selection a test edits.
func selectArea(t *testing.T, tt *ui.Tester) {
	t.Helper()
	tt.Press(100, 100)
	tt.Move(300, 250)
	tt.Release(300, 250)
}

func TestDragStartsEditing(t *testing.T) {
	s, tt := open(t, nil)
	selectArea(t, tt)

	if s.phase != phaseEdit {
		t.Fatalf("phase = %v, want the editor", s.phase)
	}
	if want := image.Rect(0, 0, 400, 300); s.crop.Bounds() != want {
		t.Errorf("crop = %v, want %v", s.crop.Bounds(), want)
	}
	if s.bar.Rect.W <= 0 {
		t.Error("the toolbar was not laid out")
	}
}

func TestDrawFinishesWithAnImage(t *testing.T) {
	var got image.Image
	var choice Choice
	s, tt := open(t, func(img image.Image, c Choice) error {
		got, choice = img, c
		return nil
	})
	selectArea(t, tt)

	tt.Press(150, 150)
	tt.Move(220, 200)
	tt.Release(220, 200)
	if s.board.Len() != 1 {
		t.Fatalf("board holds %d shapes, want 1", s.board.Len())
	}

	tt.Key(0, ui.KeyEnter)
	if got == nil {
		t.Fatal("nothing was delivered")
	}
	if want := image.Rect(0, 0, 400, 300); got.Bounds() != want {
		t.Errorf("delivered %v, want %v", got.Bounds(), want)
	}
	if choice != ChoiceCopyOnly {
		t.Errorf("choice = %v, want the clipboard", choice)
	}
	if !hasRed(got) {
		t.Error("the shape is not in the delivered image")
	}
}

// writeText types a text at pt with the text tool, as a click and Enter do.
func writeText(t *testing.T, s *selector, tt *ui.Tester, pt image.Point, text string) {
	t.Helper()
	s.tool = editor.KindText
	tt.ClickAt(float32(pt.X), float32(pt.Y))
	tt.Type(text)
	tt.Key(0, ui.KeyEnter)
	if s.typing {
		t.Fatal("the text is still being typed")
	}
}

func TestDraggingAPickedTextMovesIt(t *testing.T) {
	s, tt := open(t, nil)
	selectArea(t, tt)
	writeText(t, s, tt, image.Pt(200, 200), "挪我")
	// Longer than the double click window, so that the press picks the text
	// rather than opening it.
	tt.Advance(time.Second)
	before := s.board.Shapes()[0].Pts[0]

	tt.Press(200, 200)
	if s.selected != 0 {
		t.Fatalf("picked %d, want the text", s.selected)
	}
	tt.Move(240, 230)
	tt.Release(240, 230)

	if s.typing {
		t.Error("dragging the text started another one")
	}
	got := s.board.Shapes()[0].Pts[0]
	if got == before {
		t.Errorf("the text stayed at %v, want it moved", before)
	}
	if got.Y <= before.Y {
		t.Errorf("text moved to %v, want it lower than %v", got, before)
	}
}

func TestDoubleClickingATextTypesItAgain(t *testing.T) {
	s, tt := open(t, nil)
	selectArea(t, tt)
	writeText(t, s, tt, image.Pt(200, 200), "改这里")
	if s.board.Len() != 1 {
		t.Fatalf("board holds %d shapes, want the text", s.board.Len())
	}

	// A click picks the text, and a second one opens it.
	tt.ClickAt(200, 200)
	tt.ClickAt(200, 200)
	if !s.typing {
		t.Fatal("double clicking the text did not open it")
	}
	if s.text != "改这里" {
		t.Errorf("the field holds %q, want the text as it was", s.text)
	}
}

func TestClickingElsewhereSettlesTheText(t *testing.T) {
	s, tt := open(t, nil)
	selectArea(t, tt)

	s.tool = editor.KindText
	tt.ClickAt(200, 200)
	tt.Type("第一句")
	if !s.typing {
		t.Fatal("the click did not start a text")
	}
	tt.ClickAt(420, 420)
	if s.typing {
		t.Error("the text is still being typed after a click elsewhere")
	}
	if s.board.Len() != 1 {
		t.Fatalf("board holds %d shapes, want the text", s.board.Len())
	}
	if got := s.board.Shapes()[0].Text; got != "第一句" {
		t.Errorf("the shape holds %q, want 第一句", got)
	}
}

func TestEscapeKeepsTheTextItWasEditing(t *testing.T) {
	s, tt := open(t, nil)
	selectArea(t, tt)
	writeText(t, s, tt, image.Pt(200, 200), "别动我")

	tt.ClickAt(200, 200)
	tt.ClickAt(200, 200)
	if !s.typing {
		t.Fatal("double clicking the text did not open it")
	}
	tt.Type("改了")
	tt.Key(0, ui.KeyEscape)
	if s.typing {
		t.Error("the text is still being typed after Escape")
	}
	if s.board.Len() != 1 {
		t.Fatalf("board holds %d shapes, want the text back", s.board.Len())
	}
	if got := s.board.Shapes()[0].Text; got != "别动我" {
		t.Errorf("the shape holds %q, want the text as it was", got)
	}
}

func TestClickingPlacesNumberedBadges(t *testing.T) {
	s, tt := open(t, nil)
	selectArea(t, tt)
	s.tool = editor.KindBadge

	tt.Press(150, 150)
	tt.Release(150, 150)
	tt.Press(220, 180)
	tt.Release(220, 180)

	shapes := s.board.Shapes()
	if len(shapes) != 2 {
		t.Fatalf("board holds %d shapes, want 2 badges", len(shapes))
	}
	for i, want := range []string{"1", "2"} {
		if shapes[i].Kind != editor.KindBadge {
			t.Errorf("shape %d is kind %v, want a badge", i, shapes[i].Kind)
		}
		if shapes[i].Text != want {
			t.Errorf("badge %d shows %q, want %q", i, shapes[i].Text, want)
		}
	}
}

func TestCropToolKeepsTheDraggedPart(t *testing.T) {
	s, tt := open(t, nil)
	selectArea(t, tt)

	button, ok := s.bar.Tool(editor.KindCrop)
	if !ok {
		t.Fatal("the toolbar has no crop tool")
	}
	tt.ClickAt(button.Rect.X+button.Rect.W/2, button.Rect.Y+button.Rect.H/2)

	tt.Press(150, 150)
	tt.Move(250, 220)
	tt.Release(250, 220)

	if got, want := s.selection(), (ui.Rect{X: 150, Y: 150, W: 100, H: 70}); got != want {
		t.Errorf("selection = %+v, want %+v", got, want)
	}
	// Two capture pixels a DIP in this window.
	if got, want := s.crop.Bounds().Dx(), 200; got != want {
		t.Errorf("crop is %d wide, want %d", got, want)
	}
	if got, want := s.crop.Bounds().Dy(), 140; got != want {
		t.Errorf("crop is %d tall, want %d", got, want)
	}
	if s.tool != editor.KindRect {
		t.Errorf("the toolbar is on %v, want a drawing tool", s.tool)
	}
}

func TestToolbarUndo(t *testing.T) {
	s, tt := open(t, nil)
	selectArea(t, tt)

	tt.Press(150, 150)
	tt.Move(220, 200)
	tt.Release(220, 200)
	if s.board.Len() != 1 {
		t.Fatalf("board holds %d shapes, want 1", s.board.Len())
	}

	button, ok := s.bar.Button(editor.CommandUndo)
	if !ok {
		t.Fatal("the toolbar has no undo button")
	}
	tt.ClickAt(button.Rect.X+button.Rect.W/2, button.Rect.Y+button.Rect.H/2)
	if s.board.Len() != 0 {
		t.Errorf("undo left %d shapes", s.board.Len())
	}
}

func TestMosaicChangesTheCapture(t *testing.T) {
	var got image.Image
	s, tt := open(t, func(img image.Image, _ Choice) error { got = img; return nil })
	selectArea(t, tt)

	button, ok := s.bar.Tool(editor.KindMosaic)
	if !ok {
		t.Fatal("the toolbar has no mosaic tool")
	}
	tt.ClickAt(button.Rect.X+button.Rect.W/2, button.Rect.Y+button.Rect.H/2)
	tt.Press(150, 150)
	tt.Move(200, 180)
	tt.Release(200, 180)

	tt.Key(0, ui.KeyEnter)
	if got == nil {
		t.Fatal("nothing was delivered")
	}
	if !hasPatch(got) {
		t.Error("the mosaic left no squares in the delivered image")
	}
}

func TestEscapeCancels(t *testing.T) {
	s, tt := open(t, nil)
	tt.Key(0, ui.KeyEscape)

	select {
	case err := <-s.results:
		if !errors.Is(err, ErrCanceled) {
			t.Errorf("err = %v, want ErrCanceled", err)
		}
	default:
		t.Fatal("Escape reported nothing")
	}
}

func TestClickSelectsTheWholeScreen(t *testing.T) {
	s, tt := open(t, nil)
	tt.Press(120, 120)
	tt.Release(120, 120)

	if s.phase != phaseEdit {
		t.Fatal("a click did not start the editor")
	}
	if want := image.Rect(0, 0, 1200, 800); s.crop.Bounds() != want {
		t.Errorf("crop = %v, want the whole screen %v", s.crop.Bounds(), want)
	}
}

func TestClickSelectsTheWindowUnderThePointer(t *testing.T) {
	src := screen(1200, 800)
	window := image.Rect(200, 100, 600, 400)
	s := newSelector(Options{
		Image:   src,
		Source:  src.Bounds(),
		Place:   mygo.Rectangle{Width: 600, Height: 400},
		Windows: []Window{{Rect: window, App: "Test"}},
	}, nil)
	tt := ui.NewTester(s.view, 600, 400)

	// The window covers 100,50 to 300,200 in the window's DIPs.
	tt.Press(150, 120)
	tt.Release(150, 120)

	if s.phase != phaseEdit {
		t.Fatal("the click did not start the editor")
	}
	if got := s.crop.Bounds(); got != image.Rect(0, 0, window.Dx(), window.Dy()) {
		t.Errorf("crop = %v, want the window %dx%d", got, window.Dx(), window.Dy())
	}
}

func TestTipsShowOnHover(t *testing.T) {
	s, tt := open(t, nil)
	selectArea(t, tt)

	before := append([]uint8(nil), tt.Image().Pix...)
	button, ok := s.bar.Tool(editor.KindMosaic)
	if !ok {
		t.Fatal("the toolbar has no mosaic tool")
	}
	tt.Move(button.Rect.X+button.Rect.W/2, button.Rect.Y+button.Rect.H/2)
	tt.Advance(editor.TipDelay + 100*time.Millisecond)

	if got := tt.Image().Pix; bytes.Equal(before, got) {
		t.Fatal("resting on a button drew no tip")
	}
}

// TestRenderEditor draws the editor for KACHA_DEMO_OUT, so that the toolbar
// and the shapes can be looked at without a desktop.
func TestRenderEditor(t *testing.T) {
	out := os.Getenv("KACHA_DEMO_OUT")
	if out == "" {
		t.Skip("KACHA_DEMO_OUT is not set")
	}
	src := screen(2000, 1280)
	var delivered image.Image
	s := newSelector(Options{
		Image:  src,
		Source: src.Bounds(),
		Place:  mygo.Rectangle{Width: 1000, Height: 640},
	}, func(img image.Image, _ Choice) error { delivered = img; return nil })
	tt := ui.NewTester(s.view, 1000, 640)

	tt.Press(120, 110)
	tt.Move(700, 530)
	tt.Release(700, 530)

	steps := []struct {
		kind editor.Kind
		from [2]float32
		to   [2]float32
	}{
		{editor.KindRect, [2]float32{430, 120}, [2]float32{650, 240}},
		{editor.KindArrow, [2]float32{180, 470}, [2]float32{560, 380}},
		{editor.KindMosaic, [2]float32{250, 140}, [2]float32{395, 230}},
	}
	for _, step := range steps {
		button, ok := s.bar.Tool(step.kind)
		if !ok {
			t.Fatalf("the toolbar has no %v", step.kind)
		}
		tt.ClickAt(button.Rect.X+button.Rect.W/2, button.Rect.Y+button.Rect.H/2)
		tt.Press(step.from[0], step.from[1])
		tt.Move(step.to[0], step.to[1])
		tt.Release(step.to[0], step.to[1])
	}

	text, ok := s.bar.Tool(editor.KindText)
	if !ok {
		t.Fatal("the toolbar has no text tool")
	}
	tt.ClickAt(text.Rect.X+text.Rect.W/2, text.Rect.Y+text.Rect.H/2)
	tt.ClickAt(455, 285)
	tt.Type("改这里")
	tt.Key(0, ui.KeyEnter)

	// Two badges, to show the numbers the tool gives them.
	badge, ok := s.bar.Tool(editor.KindBadge)
	if !ok {
		t.Fatal("the toolbar has no number tool")
	}
	tt.ClickAt(badge.Rect.X+badge.Rect.W/2, badge.Rect.Y+badge.Rect.H/2)
	tt.ClickAt(462, 150)
	tt.ClickAt(545, 268)

	// Rest on a button, so that its tip shows in the picture.
	if button, ok := s.bar.Tool(editor.KindMosaic); ok {
		tt.Move(button.Rect.X+button.Rect.W/2, button.Rect.Y+button.Rect.H/2)
		tt.Advance(editor.TipDelay + 100*time.Millisecond)
	}
	if err := writePNG(out, tt.Image()); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnter)
	if delivered == nil {
		t.Fatal("nothing was delivered")
	}
	if err := writePNG(strings.TrimSuffix(out, ".png")+"-shot.png", delivered); err != nil {
		t.Fatal(err)
	}
}

// hasRed reports whether img holds a pixel of the default color.
func hasRed(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if r, g, bl, _ := img.At(x, y).RGBA(); r > 0xd000 && g < 0x6000 && bl < 0x6000 {
				return true
			}
		}
	}
	return false
}

// hasPatch reports whether img holds squares of a single color, as a mosaic
// leaves.
func hasPatch(img image.Image) bool {
	b := img.Bounds()
	// A vertical run of the same color is what a mosaic's squares make.
	same := 0
	prev := img.At(b.Min.X, b.Min.Y)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		cur := img.At(b.Min.X, y)
		if cur == prev {
			same++
			if same > editor.Block {
				return true
			}
		} else {
			same, prev = 0, cur
		}
	}
	return false
}

// writePNG writes img at path.
func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func TestTextToolWritesAnAnnotation(t *testing.T) {
	s, tt := open(t, nil)
	selectArea(t, tt)

	button, ok := s.bar.Tool(editor.KindText)
	if !ok {
		t.Fatal("the toolbar has no text tool")
	}
	tt.ClickAt(button.Rect.X+button.Rect.W/2, button.Rect.Y+button.Rect.H/2)
	tt.ClickAt(160, 160)
	if !s.typing {
		t.Fatal("the text field did not open")
	}
	tt.Type("kacha")
	tt.Key(0, ui.KeyEnter)

	if s.typing {
		t.Error("the text field stayed open")
	}
	if s.board.Len() != 1 {
		t.Fatalf("board holds %d shapes, want the text", s.board.Len())
	}
	if got := s.board.Shapes()[0]; got.Kind != editor.KindText || got.Text != "kacha" {
		t.Errorf("shape = %+v, want the text", got)
	}
}

func TestWindowKeepsItsCorners(t *testing.T) {
	src := screen(1200, 800)
	window := image.Rect(200, 100, 600, 400)
	s := newSelector(Options{
		Image:   src,
		Source:  src.Bounds(),
		Place:   mygo.Rectangle{Width: 600, Height: 400},
		Windows: []Window{{Rect: window, App: "Test", Round: true}},
	}, nil)
	tt := ui.NewTester(s.view, 600, 400)

	tt.Press(150, 120)
	tt.Release(150, 120)
	if s.phase != phaseEdit {
		t.Fatal("the click did not start the editor")
	}
	if _, _, _, a := s.crop.At(0, 0).RGBA(); a != 0 {
		t.Errorf("the top left corner is opaque: alpha %d", a)
	}
	if _, _, _, a := s.crop.At(60, 60).RGBA(); a == 0 {
		t.Error("the middle of the window is transparent")
	}
}

func TestToolbarMoves(t *testing.T) {
	src := screen(2000, 1280)
	s := newSelector(Options{
		Image:  src,
		Source: src.Bounds(),
		Place:  mygo.Rectangle{Width: 1000, Height: 640},
	}, nil)
	tt := ui.NewTester(s.view, 1000, 640)
	tt.Press(120, 110)
	tt.Move(700, 530)
	tt.Release(700, 530)

	before := s.bar.Rect
	// The bar's padding is a handle: no button is there.
	handle := image.Pt(int(before.X+2), int(before.Y+2))
	tt.Press(float32(handle.X), float32(handle.Y))
	tt.Move(float32(handle.X)+100, float32(handle.Y)-80)
	tt.Release(float32(handle.X)+100, float32(handle.Y)-80)

	if s.board.Len() != 0 {
		t.Errorf("dragging the toolbar drew %d shapes", s.board.Len())
	}
	if got := s.bar.Rect; got.X != before.X+100 || got.Y != before.Y-80 {
		t.Errorf("bar at %+v, want it moved by +100,-80 from %+v", got, before)
	}
}

func TestMenuBarIsClickable(t *testing.T) {
	src := screen(1200, 800)
	strip := image.Rect(0, 0, 1200, 40)
	s := newSelector(Options{
		Image:   src,
		Source:  src.Bounds(),
		Place:   mygo.Rectangle{Width: 600, Height: 400},
		Windows: []Window{{Rect: strip, App: "Menu Bar"}},
	}, nil)
	tt := ui.NewTester(s.view, 600, 400)

	tt.Press(300, 10)
	tt.Release(300, 10)

	if s.phase != phaseEdit {
		t.Fatal("the click did not start the editor")
	}
	if s.round != 0 {
		t.Errorf("round = %v, want square corners", s.round)
	}
	if got := s.crop.Bounds(); got != image.Rect(0, 0, 1200, 40) {
		t.Errorf("crop = %v, want the menu bar strip", got)
	}
}

func TestDraggingAnEdgeResizesTheSelection(t *testing.T) {
	s, tt := open(t, nil)
	selectArea(t, tt)
	before := s.selection()

	// Grab the right edge, halfway down the selection.
	right := before.X + before.W
	mid := before.Y + before.H/2
	tt.Press(right, mid)
	if s.resizing == 0 {
		t.Fatal("the right edge did not start a resize")
	}
	tt.Move(right-60, mid)
	tt.Release(right-60, mid)

	if got := s.selection(); got.W != before.W-60 {
		t.Errorf("width = %g, want %g", got.W, before.W-60)
	}
	if want := image.Rect(0, 0, 280, 300); s.crop.Bounds() != want {
		t.Errorf("crop = %v, want %v", s.crop.Bounds(), want)
	}
	if s.board.Len() != 0 {
		t.Errorf("resizing drew %d shapes", s.board.Len())
	}
}

func TestResizingKeepsAnnotationsInPlace(t *testing.T) {
	s, tt := open(t, nil)
	selectArea(t, tt)

	tt.Press(150, 150)
	tt.Move(220, 200)
	tt.Release(220, 200)
	if s.board.Len() != 1 {
		t.Fatal("no shape to keep")
	}
	before := s.board.Shapes()[0].Pts[0]

	sel := s.selection()
	left := sel.X
	mid := sel.Y + sel.H/2
	tt.Press(left, mid)
	tt.Move(left+40, mid)
	tt.Release(left+40, mid)

	// The capture moved 40 DIPs, so the shape moved back by as many pixels.
	if got, want := s.board.Shapes()[0].Pts[0].X, before.X-80; got != want {
		t.Errorf("shape at x=%d, want %d", got, want)
	}
	if s.crop.Bounds().Dx() != 320 {
		t.Errorf("crop width = %d, want 320", s.crop.Bounds().Dx())
	}
}
