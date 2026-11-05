// Package overlay shows a captured screen, lets the user pick a part of it,
// and edits what they picked before it leaves the window.
package overlay

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"math"
	"runtime"
	"slices"
	"strconv"
	"time"

	"github.com/douglarek/kacha/internal/editor"
	"github.com/douglarek/kacha/internal/region"
	"github.com/douglarek/kacha/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// ErrCanceled reports that the user closed the window without keeping a
// capture.
var ErrCanceled = errors.New("kacha: canceled")

// Choice is the button the user finished with.
type Choice uint8

const (
	// ChoiceBoth saves and copies, as the command line asks.
	ChoiceBoth Choice = iota
	// ChoiceCopyOnly copies without saving.
	ChoiceCopyOnly
	// ChoiceSaveOnly saves without copying.
	ChoiceSaveOnly
)

// Logf, when set, records what the overlay does, for a launch that has no
// terminal to print to.
var Logf = func(format string, args ...any) {}

// Options place the overlay over a capture.
type Options struct {
	// Image is the captured screen.
	Image image.Image
	// Source is the part of Image the window shows, in image pixels.
	Source image.Rectangle
	// Place is where the window goes, in screen DIPs.
	Place mygo.Rectangle
	// Hint is the line the overlay shows while the user drags.
	Hint string
	// Windows are the windows on screen, topmost first. A click picks one;
	// where there are none, as Wayland leaves them, a click picks the whole
	// screen.
	Windows []Window
}

// Window is one thing the user can click: a window, or the menu bar and the
// Dock, which are not windows but have rectangles of their own.
type Window struct {
	// Rect is the window in the pixels of Image.
	Rect image.Rectangle
	// App owns the window, e.g. "WeChat".
	App string
	// Round is whether the window has the rounded corners macOS gives its
	// windows, as the menu bar has none.
	Round bool
}

// Select runs the selection and the editor over o, and blocks until the user
// finishes or cancels.
// done runs on the main thread before the window closes, so that it may use
// the clipboard; its error is returned.
func Select(o Options, done func(image.Image, Choice) error) error {
	return run(newSelector(o, done))
}

// run opens the window of s and waits until the user finishes or cancels.
func run(s *selector) error {
	o := s.o
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:           "Kacha",
		Frameless:       true,
		AlwaysOnTop:     true,
		SkipTaskbar:     true,
		DisableResize:   true,
		DisableMove:     true,
		DisableMinimize: true,
		DisableMaximize: true,
		DisableClose:    true,
		BackgroundColor: "#000000",
		X:               o.Place.X,
		Y:               o.Place.Y,
		Width:           o.Place.Width,
		Height:          o.Place.Height,
		FullScreen:      runtime.GOOS == "linux",
		Content:         ui.View(s.view),
	})
	s.win = w
	Logf("overlay: window %dx%d at %d,%d", o.Place.Width, o.Place.Height, o.Place.X, o.Place.Y)
	w.Show()
	w.Focus()
	coverMenuBar(w)
	if nudge {
		w.SetSize(o.Place.Width, o.Place.Height)
		w.SetPosition(o.Place.X, o.Place.Y)
	}
	select {
	case err := <-s.results:
		return err
	case <-time.After(idleLimit):
		w.Close()
		return ErrCanceled
	}
}

// nudge moves the window again after coverMenuBar has lifted it: macOS keeps
// an ordinary window below the menu bar, and only a window above it stays
// where it was asked to go.
const nudge = runtime.GOOS == "darwin"

// idleLimit ends an overlay that a compositor never lets anyone finish.
const idleLimit = 10 * time.Minute

// maxTrail bounds the points one jump of the pointer adds to a trail.
const maxTrail = 512

// phase is what the overlay waits for.
type phase uint8

const (
	phaseSelect phase = iota
	phaseEdit
)

// selector is the state of one overlay, read by the view on the main thread.
type selector struct {
	o       Options
	bitmap  *ui.Bitmap
	done    func(image.Image, Choice) error
	results chan error
	win     *mygo.Window

	// box is the window's drawing area as the last frame laid it out.
	box region.Rect
	// from and to are the corners of the selection, in the window's DIPs.
	from, to [2]float32
	hasFrom  bool
	// at is where the pointer is, and hasAt whether it is over the window.
	at    [2]float32
	hasAt bool
	// pressed is whether the pointer went down and did not come back up,
	// onButton whether it went down on the toolbar.
	pressed   bool
	onButton  bool
	pressedOn ui.Rect
	// barDragging is whether the pointer moves the toolbar, and barShift how
	// far the user moved it from where the selection put it.
	barDragging bool
	barShift    [2]float32
	lastAt      [2]float32

	// hover is the button the pointer rests on, and hoverAt when it got there.
	hover   ui.Rect
	hoverAt time.Time

	phase  phase
	crop   image.Image
	mosaic *editor.Mosaic
	board  editor.Board
	tool   editor.Kind
	color  ui.Color
	width  float32
	bar    editor.Bar

	// round is the corner radius, in DIPs, of the window the selection came
	// from, 0 for a selection the user dragged.
	round float64
	// cropAt is where the crop came from, in the pixels of the capture, and
	// resizing which edges of the selection the pointer drags.
	cropAt   image.Rectangle
	resizing edges

	drafting bool
	draft    editor.Shape

	typing bool
	textAt image.Point
	text   string
	// edited is the text a double click took off the board to type again,
	// nil for a text that is new.
	edited *editor.Shape
	// cropping is whether the pointer drags a new crop, from cropFrom to
	// cropTo, in the window's DIPs.
	cropping bool
	cropFrom ui.Point
	cropTo   ui.Point
	// selected is the text the pointer picked, -1 for none, and moving
	// whether the pointer drags it. moveFrom is where the drag began, and
	// moveShape the text as it was before it.
	selected  int
	moving    bool
	moveFrom  ui.Point
	moveShape editor.Shape

	finished bool
}

// asRGBA returns img as the premultiplied RGBA whose pixels mygo shows as
// they are: it copies any other image, so the capture goes as one, and the
// window and the app share its pixels instead of holding one copy each.
func asRGBA(img image.Image) image.Image {
	if rgba, ok := img.(*image.RGBA); ok && rgba.Bounds().Min == (image.Point{}) && rgba.Stride == 4*rgba.Bounds().Dx() {
		return rgba
	}
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

func newSelector(o Options, done func(image.Image, Choice) error) *selector {
	if o.Image != nil {
		o.Image = asRGBA(o.Image)
	}
	return &selector{
		o:        o,
		bitmap:   ui.NewBitmap(o.Image),
		done:     done,
		results:  make(chan error, 1),
		tool:     editor.KindRect,
		color:    editor.Colors()[0],
		width:    editor.Widths()[0],
		selected: -1,
	}
}

// view lays the overlay out and reads the pointer and the keys, once a frame.
func (s *selector) view(c *ui.Context) {
	if s.finished {
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		if s.typing {
			// The text an escape leaves was there before, and stays.
			if s.edited != nil {
				s.board.Add(*s.edited)
			}
			s.typing, s.text, s.edited = false, "", nil
			return
		}
		s.finish(nil, ChoiceBoth, ErrCanceled)
		return
	}
	if s.phase == phaseEdit && !s.typing {
		if c.Shortcut(ui.Cmd, ui.KeyZ) {
			s.board.Undo()
		}
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyZ) {
			s.board.Redo()
		}
		if c.Shortcut(0, ui.KeyEnter) {
			s.commit(ChoiceCopyOnly)
			return
		}
	}

	root := ui.Box(c).Fill()
	root.Draw(func(p *ui.Painter, r ui.Rect) {
		s.box = region.Rect{X: float64(r.X), Y: float64(r.Y), W: float64(r.W), H: float64(r.H)}
		if s.phase == phaseSelect {
			s.paintSelect(p, r)
		} else {
			s.paintEdit(p, r)
		}
	})
	if s.phase == phaseEdit && s.typing {
		s.textField(c, root)
		return
	}
	if root.RightClicked() {
		s.finish(nil, ChoiceBoth, ErrCanceled)
		return
	}
	if s.phase == phaseEdit && root.DoubleClicked() {
		// A double click on a text types it again; anywhere else it ends the
		// capture.
		if x, y, over := root.PointerPosition(); over {
			if i, ok := s.textUnder(ui.Point{X: x, Y: y}); ok {
				s.editText(i)
				return
			}
		}
		s.commit(ChoiceCopyOnly)
		return
	}
	if s.phase == phaseSelect {
		s.trackSelection(root)
	} else {
		s.trackEditing(root)
	}
}

// trackSelection reads the drag that picks the part to keep.
func (s *selector) trackSelection(root ui.Element) {
	x, y, over := root.PointerPosition()
	s.at, s.hasAt = [2]float32{x, y}, over
	pressed := root.Pressed() && over
	switch {
	case pressed && !s.hasFrom:
		s.from, s.to = [2]float32{x, y}, [2]float32{x, y}
		s.hasFrom = true
	case pressed:
		s.to = [2]float32{x, y}
	case s.hasFrom:
		if !s.draggedFar(x, y) {
			s.pick(ui.Point{X: x, Y: y})
		}
		if s.cropRect().Empty() {
			s.finish(nil, ChoiceBoth, ErrCanceled)
			return
		}
		s.startEditing()
	}
}

// trackEditing reads the pointer in the editor: the toolbar, a tool, or a
// shape being dragged out.
func (s *selector) trackEditing(root ui.Element) {
	x, y, over := root.PointerPosition()
	s.at, s.hasAt = [2]float32{x, y}, over
	pt := ui.Point{X: x, Y: y}
	down := root.Pressed() && over
	switch {
	case down && !s.pressed:
		s.pressed = true
		s.onButton = false
		if button, ok := s.bar.Hit(pt); ok && !button.Disabled {
			s.onButton = true
			s.pressedOn = button.Rect
			return
		}
		if s.bar.Holds(pt) {
			s.barDragging = true
			s.lastAt = [2]float32{x, y}
			return
		}
		if s.tool == editor.KindCrop {
			s.cropping, s.cropFrom, s.cropTo = true, pt, pt
			return
		}
		if e, ok := s.edgeAt(pt); ok {
			s.resizing = e
			return
		}
		if s.selection().Contains(x, y) {
			s.beginShape(pt)
		}
	case down && s.pressed:
		if s.barDragging {
			s.barShift[0] += x - s.lastAt[0]
			s.barShift[1] += y - s.lastAt[1]
			s.lastAt = [2]float32{x, y}
			return
		}
		if s.resizing != 0 {
			s.resizeTo(pt)
			return
		}
		if s.cropping {
			s.cropTo = pt
			return
		}
		if s.moving {
			s.moveText(pt)
			return
		}
		if s.drafting {
			s.extendShape(pt)
		}
	case s.pressed:
		s.pressed = false
		s.barDragging = false
		if s.resizing != 0 {
			s.resizing = 0
			s.rebuild()
			return
		}
		if s.cropping {
			s.cropping = false
			s.applyCrop()
			return
		}
		if s.moving {
			s.moving = false
			return
		}
		if s.onButton {
			s.onButton = false
			if button, ok := s.bar.Hit(pt); ok && button.Rect == s.pressedOn {
				s.act(button.Action)
			}
			return
		}
		s.commitShape()
	}
}

// pickText selects the text at index, which the drag beginning at pt moves.
func (s *selector) pickText(index int, pt ui.Point) {
	shapes := s.board.Shapes()
	if index < 0 || index >= len(shapes) {
		return
	}
	s.selected = index
	s.moving = true
	s.moveFrom = pt
	s.moveShape = shapes[index]
}

// moveText moves the text the pointer picked along with it, kept inside the
// capture.
func (s *selector) moveText(pt ui.Point) {
	if !s.moving || s.selected < 0 || len(s.moveShape.Pts) == 0 {
		return
	}
	k := s.scale()
	dx := int((pt.X - s.moveFrom.X) * k)
	dy := int((pt.Y - s.moveFrom.Y) * k)
	moved := s.moveShape
	moved.Pts = slices.Clone(s.moveShape.Pts)
	moved.Pts[0] = s.clampText(image.Pt(
		s.moveShape.Pts[0].X+dx,
		s.moveShape.Pts[0].Y+dy,
	))
	s.board.Replace(s.selected, moved)
}

// act runs what a button of the toolbar does.
func (s *selector) act(a editor.Action) {
	switch a.Kind {
	case editor.ActionTool:
		s.tool = a.Tool
	case editor.ActionColor:
		s.color = a.Color
	case editor.ActionWidth:
		s.width = a.Width
	case editor.ActionCommand:
		s.command(a.Command)
	}
}

// command runs one of the toolbar's commands.
func (s *selector) command(cmd editor.Command) {
	switch cmd {
	case editor.CommandUndo:
		s.board.Undo()
		s.selected = -1
	case editor.CommandRedo:
		s.board.Redo()
		s.selected = -1
	case editor.CommandCancel:
		s.finish(nil, ChoiceBoth, ErrCanceled)
	case editor.CommandSave:
		s.commit(ChoiceSaveOnly)
	case editor.CommandCopy:
		s.commit(ChoiceCopyOnly)
	}
}

// startEditing crops what the drag picked and hands it to the editor.
func (s *selector) startEditing() {
	r := s.cropRect()
	img := cropImage(s.o.Image, r)
	if sel := s.selection(); s.round > 0 && sel.W > 0 {
		roundCrop(img, s.round*float64(r.Dx())/float64(sel.W))
	}
	s.cropAt = r
	s.crop = img
	s.mosaic = editor.NewMosaic(img)
	s.phase = phaseEdit
}

// edges says which sides of the selection a drag moves.
type edges uint8

const (
	edgeLeft edges = 1 << iota
	edgeRight
	edgeTop
	edgeBottom
)

// grab is how close to a side of the selection the pointer resizes it.
const grab = 8

// minSelection is the smallest selection the pointer may drag out, in DIPs.
const minSelection = 8

// edgeAt returns the sides of the selection that p is close enough to move.
func (s *selector) edgeAt(p ui.Point) (e edges, ok bool) {
	sel := s.selection()
	if sel.W <= 0 {
		return 0, false
	}
	near := func(v, side float32) bool { return v >= side-grab && v <= side+grab }
	x0, y0 := sel.X, sel.Y
	x1, y1 := sel.X+sel.W, sel.Y+sel.H
	switch {
	case near(p.X, x0) && p.Y >= y0-grab && p.Y <= y1+grab:
		e |= edgeLeft
	case near(p.X, x1) && p.Y >= y0-grab && p.Y <= y1+grab:
		e |= edgeRight
	}
	switch {
	case near(p.Y, y0) && p.X >= x0-grab && p.X <= x1+grab:
		e |= edgeTop
	case near(p.Y, y1) && p.X >= x0-grab && p.X <= x1+grab:
		e |= edgeBottom
	}
	return e, e != 0
}

// resizeTo moves the sides of the selection that the pointer drags to p.
func (s *selector) resizeTo(p ui.Point) {
	r := s.rawSelection()
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	if s.resizing&edgeLeft != 0 {
		x0 = min(float64(p.X), x1-minSelection)
	}
	if s.resizing&edgeRight != 0 {
		x1 = max(float64(p.X), x0+minSelection)
	}
	if s.resizing&edgeTop != 0 {
		y0 = min(float64(p.Y), y1-minSelection)
	}
	if s.resizing&edgeBottom != 0 {
		y1 = max(float64(p.Y), y0+minSelection)
	}
	box := s.windowBox()
	x0, x1 = max(x0, float64(box.X)), min(x1, float64(box.X+box.W))
	y0, y1 = max(y0, float64(box.Y)), min(y1, float64(box.Y+box.H))
	s.from, s.to = [2]float32{float32(x0), float32(y0)}, [2]float32{float32(x1), float32(y1)}
	s.round = 0
}

// rebuild crops the selection again after the pointer resized it, keeping
// the annotations where they are on the capture.
func (s *selector) rebuild() {
	at := s.cropRect()
	if at == s.cropAt || at.Empty() {
		return
	}
	s.board.Shift(s.cropAt.Min.X-at.Min.X, s.cropAt.Min.Y-at.Min.Y)
	s.cropAt = at
	img := cropImage(s.o.Image, at)
	s.crop = img
	s.mosaic = editor.NewMosaic(img)
}

// draggedFar reports whether the drag the pointer made is worth more than a
// click.
func (s *selector) draggedFar(x, y float32) bool {
	return max(x-s.from[0], s.from[0]-x) > 3 || max(y-s.from[1], s.from[1]-y) > 3
}

// pick takes the window at p, or the whole screen where there is none.
func (s *selector) pick(p ui.Point) {
	box := s.windowBox()
	s.from, s.to = [2]float32{box.X, box.Y}, [2]float32{box.X + box.W, box.Y + box.H}
	s.round = 0
	if window, ok := s.windowAt(p); ok {
		r := s.sourceToWindow(window.Rect)
		s.from, s.to = [2]float32{r.X, r.Y}, [2]float32{r.X + r.W, r.Y + r.H}
		if window.Round {
			s.round = windowRadius
		}
	}
}

// windowAt returns the topmost window under p of the window's DIPs, in the
// pixels of the capture.
func (s *selector) windowAt(p ui.Point) (Window, bool) {
	at := s.toSource(p)
	for _, window := range s.o.Windows {
		if at.In(window.Rect) {
			return window, true
		}
	}
	return Window{}, false
}

// toSource returns where p of the window falls in the capture the window
// shows.
func (s *selector) toSource(p ui.Point) image.Point {
	box := s.windowBox()
	if box.W <= 0 || s.o.Source.Empty() {
		return image.Point{}
	}
	k := float64(s.o.Source.Dx()) / float64(box.W)
	return image.Pt(
		s.o.Source.Min.X+int((float64(p.X)-float64(box.X))*k),
		s.o.Source.Min.Y+int((float64(p.Y)-float64(box.Y))*k),
	)
}

// sourceToWindow returns r of the capture in the window's DIPs.
func (s *selector) sourceToWindow(r image.Rectangle) ui.Rect {
	box := s.windowBox()
	k := box.W / float32(max(s.o.Source.Dx(), 1))
	return ui.Rect{
		X: box.X + float32(r.Min.X-s.o.Source.Min.X)*k,
		Y: box.Y + float32(r.Min.Y-s.o.Source.Min.Y)*k,
		W: float32(r.Dx()) * k,
		H: float32(r.Dy()) * k,
	}
}

// beginShape starts what the tool draws at pt, which is in the window's DIPs.
func (s *selector) beginShape(pt ui.Point) {
	at := s.toCapture(pt)
	if s.tool == editor.KindText {
		// A text that is already there is picked by the click, and the drag
		// that may follow moves it, rather than a second one being written
		// over it.
		if i, ok := s.textUnder(pt); ok {
			s.pickText(i, pt)
			return
		}
		s.selected = -1
		s.edited = nil
		s.typing, s.textAt, s.text = true, at, ""
		s.pressed = false
		return
	}
	if s.tool == editor.KindBadge {
		// A badge is placed rather than dragged, and numbers what came
		// before it. The press stays on, so that the pointer going down
		// again next frame does not place a second one.
		s.board.Add(editor.Shape{
			Kind:  editor.KindBadge,
			Color: s.color,
			Width: s.width * 12,
			Pts:   []image.Point{at},
			Text:  strconv.Itoa(s.board.NextBadge()),
		})
		return
	}
	s.drafting = true
	width := s.width * s.scale()
	if s.tool == editor.KindMosaic {
		// The mosaic is a brush wider than a stroke, as the eraser is.
		width *= 4
	}
	s.draft = editor.Shape{
		Kind:  s.tool,
		Color: s.color,
		Width: width,
		Pts:   []image.Point{at, at},
	}
}

// extendShape keeps what the tool draws up with the pointer.
func (s *selector) extendShape(pt ui.Point) {
	if !s.drafting || len(s.draft.Pts) == 0 {
		return
	}
	at := s.toCapture(pt)
	switch s.draft.Kind {
	case editor.KindPen, editor.KindMosaic:
		last := s.draft.Pts[len(s.draft.Pts)-1]
		dx, dy := at.X-last.X, at.Y-last.Y
		if max(dx, -dx) < 2 && max(dy, -dy) < 2 {
			return
		}
		// A pointer that jumps leaves a trail: fill the gap a step at a time.
		step := max(s.draft.Width/2, 2)
		gap := int(math.Hypot(float64(dx), float64(dy)) / float64(step))
		for i := 1; i <= min(gap, maxTrail); i++ {
			t := float64(i) / float64(gap+1)
			s.draft.Pts = append(s.draft.Pts, image.Pt(
				last.X+int(float64(dx)*t),
				last.Y+int(float64(dy)*t),
			))
		}
		s.draft.Pts = append(s.draft.Pts, at)
	default:
		s.draft.Pts[1] = at
	}
}

// commitShape puts the shape being drawn on the board.
func (s *selector) commitShape() {
	if !s.drafting {
		return
	}
	s.board.Add(s.draft)
	s.selected = -1
	s.drafting = false
	s.draft = editor.Shape{}
}

// commitText puts the text the user typed on the board.
func (s *selector) commitText() {
	if s.text != "" {
		width := s.width * 12
		if s.edited != nil {
			// The text keeps the size it was typed at.
			width = s.edited.Width
		}
		s.board.Add(editor.Shape{
			Kind:  editor.KindText,
			Color: s.color,
			Width: width,
			Pts:   []image.Point{s.textAt},
			Text:  s.text,
		})
	}
	s.typing, s.text, s.edited, s.selected = false, "", nil, -1
}

// commit renders the capture with its annotations and finishes.
func (s *selector) commit(choice Choice) {
	if s.phase != phaseEdit || s.crop == nil {
		s.finish(nil, choice, ErrCanceled)
		return
	}
	s.commitShape()
	s.finish(editor.Render(s.crop, &s.board, s.mosaic), choice, nil)
}

// finish reports the outcome and closes the window.
func (s *selector) finish(img image.Image, choice Choice, err error) {
	if s.finished {
		return
	}
	Logf("overlay: finish choice=%v err=%v phase=%v", choice, err, s.phase)
	s.finished = true
	if err == nil && img != nil && s.done != nil {
		err = s.done(img, choice)
	}
	s.results <- err
	if s.win != nil {
		s.win.Close()
		s.win = nil
	}
	// The capture and its frames are large, and spent: the app that stays
	// for the next one should not keep them.
	s.o.Image = nil
	s.bitmap = nil
	s.crop = nil
	s.mosaic = nil
	s.done = nil
}

// textField shows the field a text annotation is typed into.
// textField types a text over the capture. The field has a look of its own
// rather than the theme's: it stands over the capture, which it keeps visible
// through a background that shows what the text is about to cover.
func (s *selector) textField(c *ui.Context, root ui.Element) {
	x, y := s.toWindow(s.textAt)
	size := s.textSize()
	const width = 240
	settle := false
	ui.Overlay(c, func() {
		field := ui.TextInputBase(c, &s.text)
		field.Absolute().Left(x - 6).Top(y - size*0.2).Width(width).AutoFocus()
		field.Padding(size*0.18, 7).Radius(6).FontSize(size)
		field.Background(ui.RGBA(18, 18, 22, 0.42))
		field.Border(1, ui.RGBA(255, 255, 255, 0.18))
		field.TextColor(ui.RGB(255, 255, 255))
		switch {
		case field.Submitted():
			settle = true
		case field.PressedOutside():
			// A press anywhere else settles the text, as Enter does.
			settle = true
		}
	})
	if settle {
		s.commitText()
	}
}

// clampText keeps a text's corner inside the capture.
func (s *selector) clampText(at image.Point) image.Point {
	bounds := s.crop.Bounds()
	at.X = min(max(at.X, 0), bounds.Dx())
	at.Y = min(max(at.Y, 0), bounds.Dy())
	return at
}

// textSize returns the height of the glyphs of the text being typed, in the
// window's DIPs: the one the text had, or the one the toolbar gives.
func (s *selector) textSize() float32 {
	width := s.width * 12
	if s.edited != nil {
		width = s.edited.Width
	}
	return width / s.scale()
}

// textUnder returns the index of the text under p, in the window's DIPs.
func (s *selector) textUnder(p ui.Point) (int, bool) {
	for i, shape := range s.board.Shapes() {
		if shape.Kind != editor.KindText || shape.Text == "" {
			continue
		}
		if s.textBox(shape).Contains(p.X, p.Y) {
			return i, true
		}
	}
	return 0, false
}

// textBox returns where a text shows in the window.
func (s *selector) textBox(shape editor.Shape) ui.Rect {
	if len(shape.Pts) == 0 {
		return ui.Rect{}
	}
	w, h := editor.TextExtent(shape)
	x, y := s.toWindow(shape.Pts[0])
	k := s.scale()
	return ui.Rect{X: x, Y: y, W: w / k, H: h / k}
}

// editText takes the text at index off the board and types it again, at the
// size and in the color it was written.
func (s *selector) editText(index int) {
	shape, ok := s.board.Take(index)
	if !ok || shape.Kind != editor.KindText {
		return
	}
	s.edited = &shape
	s.selected = -1
	s.moving = false
	s.typing = true
	s.textAt = shape.Pts[0]
	s.text = shape.Text
	s.color = shape.Color
}

// paintSelect draws the capture, dims what is not selected, and labels the
// selection with the size it has in pixels.
func (s *selector) paintSelect(p *ui.Painter, r ui.Rect) {
	p.ImageRegion(s.bitmap, s.sourceRect(), r, ui.ImageOptions{})
	dim := ui.RGBA(0, 0, 0, 0.45)
	sel, selected := s.dragged()
	if !selected {
		if s.paintWindowHover(p, r, dim) {
			return
		}
		p.Fill(r, dim, 0)
		s.paintCrosshair(p)
		s.paintHint(p, r)
		return
	}
	s.dimOutside(p, r, sel, 0, dim)
	p.Stroke(sel, ui.RGB(52, 199, 89), 0, 2.5)
	s.paintSize(p, sel)
	s.paintHint(p, r)
}

// paintEdit draws the capture, its annotations and the toolbar.
func (s *selector) paintEdit(p *ui.Painter, r ui.Rect) {
	sel := s.selection()
	p.ImageRegion(s.bitmap, s.sourceRect(), r, ui.ImageOptions{})
	if s.cropping {
		// The drag picks what is left of the capture, so it is what stays lit.
		draft := s.cropDraft()
		s.dimOutside(p, r, draft, 0, ui.RGBA(0, 0, 0, 0.55))
		p.Stroke(draft, ui.RGB(52, 199, 89), 0, 2)
	} else {
		s.dimOutside(p, r, sel, float32(s.round), ui.RGBA(0, 0, 0, 0.45))
	}

	shapes := slices.Clone(s.board.Shapes())
	if s.drafting {
		shapes = append(shapes, s.draft)
	}
	editor.Paint(p, shapes, s.mosaic, 1/s.scale(), sel)
	if !s.cropping {
		p.Stroke(sel, ui.RGB(52, 199, 89), float32(s.round), 2.5)
		s.paintHandles(p, sel)
		s.paintGrab(p, sel)
	}
	s.paintPicked(p)

	s.bar = editor.Layout(sel, r, editor.State{
		Tool:    s.tool,
		Color:   s.color,
		Width:   s.width,
		CanUndo: s.board.Len() > 0,
		CanRedo: s.board.CanRedo(),
	})
	s.placeBar(r)
	s.bar.Draw(p, ui.Point{X: s.at[0], Y: s.at[1]}, s.hasAt)
	s.paintTip(p, r)
}

// paintGrab shows which side of the selection the pointer would move.
func (s *selector) paintGrab(p *ui.Painter, sel ui.Rect) {
	if !s.hasAt || s.pressed || s.bar.Holds(ui.Point{X: s.at[0], Y: s.at[1]}) {
		return
	}
	e, ok := s.edgeAt(ui.Point{X: s.at[0], Y: s.at[1]})
	if !ok {
		return
	}
	const thick = float32(4)
	accent := ui.RGB(52, 199, 89)
	if e&edgeLeft != 0 {
		p.Fill(ui.Rect{X: sel.X - thick/2, Y: sel.Y, W: thick, H: sel.H}, accent, thick/2)
	}
	if e&edgeRight != 0 {
		p.Fill(ui.Rect{X: sel.X + sel.W - thick/2, Y: sel.Y, W: thick, H: sel.H}, accent, thick/2)
	}
	if e&edgeTop != 0 {
		p.Fill(ui.Rect{X: sel.X, Y: sel.Y - thick/2, W: sel.W, H: thick}, accent, thick/2)
	}
	if e&edgeBottom != 0 {
		p.Fill(ui.Rect{X: sel.X, Y: sel.Y + sel.H - thick/2, W: sel.W, H: thick}, accent, thick/2)
	}
}

// paintHandles marks the corners of the selection: the pointer drags one to
// resize what was picked, which the grab at an edge does not show until the
// pointer is on it.
func (s *selector) paintHandles(p *ui.Painter, sel ui.Rect) {
	const radius = float32(4.5)
	accent := ui.RGB(52, 199, 89)
	for _, corner := range [][2]float32{
		{sel.X, sel.Y},
		{sel.X + sel.W, sel.Y},
		{sel.X, sel.Y + sel.H},
		{sel.X + sel.W, sel.Y + sel.H},
	} {
		p.Fill(ui.Rect{
			X: corner[0] - radius, Y: corner[1] - radius,
			W: 2 * radius, H: 2 * radius,
		}, accent, radius)
	}
}

// placeBar moves the toolbar where the user dragged it, kept inside r.
func (s *selector) placeBar(r ui.Rect) {
	const margin = float32(4)
	loX := r.X + margin - s.bar.Rect.X
	hiX := max(r.X+r.W-margin-s.bar.Rect.W-s.bar.Rect.X, loX)
	loY := r.Y + margin - s.bar.Rect.Y
	hiY := max(r.Y+r.H-margin-s.bar.Rect.H-s.bar.Rect.Y, loY)
	s.barShift[0] = min(max(s.barShift[0], loX), hiX)
	s.barShift[1] = min(max(s.barShift[1], loY), hiY)
	s.bar.Move(s.barShift[0], s.barShift[1])
}

// paintTip names the button the pointer rests on.
func (s *selector) paintTip(p *ui.Painter, r ui.Rect) {
	if !s.hasAt || s.drafting {
		s.hover = ui.Rect{}
		return
	}
	button, ok := s.bar.Hit(ui.Point{X: s.at[0], Y: s.at[1]})
	if !ok || button.Tip == "" {
		s.hover = ui.Rect{}
		return
	}
	if button.Rect != s.hover {
		s.hover, s.hoverAt = button.Rect, p.Now()
	}
	if wait := editor.TipDelay - p.Now().Sub(s.hoverAt); wait > 0 {
		p.After(wait)
		return
	}
	editor.DrawTip(p, button.Rect, button.Tip, r)
}

// dimOutside dims the window around sel, whose corners are rounded by
// radius, and shows what sel keeps as it is.
func (s *selector) dimOutside(p *ui.Painter, r, sel ui.Rect, radius float32, dim ui.Color) {
	p.Fill(r, dim, 0)
	p.Clip(sel, radius, func() {
		p.ImageRegion(s.bitmap, s.sourceRect(), r, ui.ImageOptions{})
	})
}

// paintWindowHover marks the window a click would pick, and reports
// whether it drew one.
func (s *selector) paintWindowHover(p *ui.Painter, r ui.Rect, dim ui.Color) bool {
	if !s.hasAt || s.hasFrom {
		return false
	}
	window, ok := s.windowAt(ui.Point{X: s.at[0], Y: s.at[1]})
	if !ok {
		return false
	}
	hover := s.sourceToWindow(window.Rect)
	radius := float32(0)
	if window.Round {
		radius = windowRadius
	}
	// What a click would take stays bright; the rest dims.
	s.dimOutside(p, r, hover, radius, dim)
	p.Stroke(hover, ui.RGB(52, 199, 89), radius, 2.5)
	if window.App != "" {
		editor.DrawTip(p, hover, window.App, r)
	}
	return true
}

// paintCrosshair marks where the pointer is, so that the selection starts
// where the eye expects it to.
func (s *selector) paintCrosshair(p *ui.Painter) {
	if !s.hasAt {
		return
	}
	box := s.windowBox()
	line := ui.RGBA(255, 255, 255, 0.55)
	p.Line(box.X, s.at[1], box.X+box.W, s.at[1], 1, line)
	p.Line(s.at[0], box.Y, s.at[0], box.Y+box.H, 1, line)
}

// paintSize labels the selection with its size in the capture's pixels.
func (s *selector) paintSize(p *ui.Painter, sel ui.Rect) {
	box := s.windowBox()
	px := region.Mapping{Source: s.o.Source, Dst: s.box}.Pixels(region.Rect{
		X: float64(sel.X), Y: float64(sel.Y), W: float64(sel.W), H: float64(sel.H),
	})
	label := fmt.Sprintf("%d × %d", px.Dx(), px.Dy())
	const size, pad = float32(12), float32(6)
	w, h := p.MeasureText(0, ui.Span{Text: label, Size: size})
	x := min(sel.X+sel.W-w-pad, box.X+box.W-w-pad)
	y := sel.Y + sel.H + pad
	if y+h+2*pad > box.Y+box.H {
		y = sel.Y - h - 2*pad
	}
	p.Fill(ui.Rect{X: x - pad, Y: y, W: w + 2*pad, H: h + 2*pad}, ui.RGBA(0, 0, 0, 0.75), 4)
	p.Text(x, y+pad, label, size, ui.RGB(255, 255, 255))
}

// paintHint shows what the pointer does and what the keys mean.
func (s *selector) paintHint(p *ui.Painter, r ui.Rect) {
	hint := s.o.Hint
	if hint == "" {
		hint = locales.Hint()
	}
	const size, pad = float32(11), float32(7)
	w, h := p.MeasureText(0, ui.Span{Text: hint, Size: size})
	x := r.X + (r.W-w)/2
	y := r.Y + 24
	p.Fill(ui.Rect{X: x - pad, Y: y - pad, W: w + 2*pad, H: h + 2*pad}, ui.RGBA(0, 0, 0, 0.70), 6)
	p.Text(x, y, hint, size, ui.RGB(255, 255, 255))
}

// dragged returns the rectangle being dragged, in the window's DIPs.
func (s *selector) dragged() (ui.Rect, bool) {
	if !s.hasFrom {
		return ui.Rect{}, false
	}
	r := s.rawSelection()
	if r.Empty() {
		return ui.Rect{}, false
	}
	return ui.Rect{X: float32(r.X), Y: float32(r.Y), W: float32(r.W), H: float32(r.H)}, true
}

// selection returns the rectangle the editor works in, in the window's DIPs.
func (s *selector) selection() ui.Rect {
	sel, _ := s.dragged()
	return sel
}

// rawSelection returns the corners dragged, clamped to the window.
func (s *selector) rawSelection() region.Rect {
	if !s.hasFrom {
		return region.Rect{}
	}
	return region.FromCorners(
		float64(s.from[0]), float64(s.from[1]),
		float64(s.to[0]), float64(s.to[1]),
	).ClampTo(s.box)
}

// cropRect returns the part of the capture the selection keeps.
func (s *selector) cropRect() image.Rectangle {
	if !s.hasFrom {
		return image.Rectangle{}
	}
	return region.Mapping{Source: s.o.Source, Dst: s.box}.Pixels(s.rawSelection())
}

// scale returns how many pixels of the capture a DIP of the window holds.
func (s *selector) scale() float32 {
	sel := s.selection()
	if sel.W <= 0 || s.crop == nil {
		return 1
	}
	return float32(s.crop.Bounds().Dx()) / sel.W
}

// toCapture returns where p of the window falls in the capture.
func (s *selector) toCapture(p ui.Point) image.Point {
	sel := s.selection()
	k := s.scale()
	at := image.Pt(
		int((p.X-sel.X)*k),
		int((p.Y-sel.Y)*k),
	)
	if s.crop != nil {
		at.X = min(max(at.X, 0), s.crop.Bounds().Dx())
		at.Y = min(max(at.Y, 0), s.crop.Bounds().Dy())
	}
	return at
}

// toWindow returns where a point of the capture falls in the window.
func (s *selector) toWindow(pt image.Point) (float32, float32) {
	sel := s.selection()
	k := s.scale()
	return sel.X + float32(pt.X)/k, sel.Y + float32(pt.Y)/k
}

// sourceRect returns the part of the bitmap the window shows.
func (s *selector) sourceRect() ui.Rect {
	return ui.Rect{
		X: float32(s.o.Source.Min.X), Y: float32(s.o.Source.Min.Y),
		W: float32(s.o.Source.Dx()), H: float32(s.o.Source.Dy()),
	}
}

// windowBox returns the layout box as a ui.Rect.
func (s *selector) windowBox() ui.Rect {
	return ui.Rect{
		X: float32(s.box.X), Y: float32(s.box.Y),
		W: float32(s.box.W), H: float32(s.box.H),
	}
}

// cropImage returns the part of img inside r.
func cropImage(img image.Image, r image.Rectangle) *image.RGBA {
	r = r.Intersect(img.Bounds())
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), img, r.Min, draw.Src)
	return out
}

// paintPicked outlines the text the pointer picked, which a drag moves.
func (s *selector) paintPicked(p *ui.Painter) {
	if s.selected < 0 || s.typing {
		return
	}
	shapes := s.board.Shapes()
	if s.selected >= len(shapes) {
		return
	}
	box := s.textBox(shapes[s.selected])
	p.Stroke(ui.Rect{
		X: box.X - 3, Y: box.Y - 3, W: box.W + 6, H: box.H + 6,
	}, ui.RGB(52, 199, 89), 0, 1.5)
}

// cropDraft returns the rectangle the pointer drags with the crop tool, in
// the window's DIPs.
func (s *selector) cropDraft() ui.Rect {
	x0, x1 := min(s.cropFrom.X, s.cropTo.X), max(s.cropFrom.X, s.cropTo.X)
	y0, y1 := min(s.cropFrom.Y, s.cropTo.Y), max(s.cropFrom.Y, s.cropTo.Y)
	return ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// applyCrop keeps the part of the capture the crop tool dragged over, and
// leaves the toolbar on a tool that draws.
func (s *selector) applyCrop() {
	draft := s.cropDraft()
	if draft.W < minSelection || draft.H < minSelection {
		return
	}
	box := s.windowBox()
	clampX := func(v float32) float32 { return min(max(v, box.X), box.X+box.W) }
	clampY := func(v float32) float32 { return min(max(v, box.Y), box.Y+box.H) }
	x0, x1 := clampX(draft.X), clampX(draft.X+draft.W)
	y0, y1 := clampY(draft.Y), clampY(draft.Y+draft.H)
	s.from, s.to = [2]float32{float32(x0), float32(y0)}, [2]float32{float32(x1), float32(y1)}
	s.round = 0
	s.rebuild()
	s.tool = editor.KindRect
	s.selected = -1
}
