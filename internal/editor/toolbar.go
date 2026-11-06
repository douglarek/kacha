package editor

import (
	"time"

	"github.com/douglarek/kacha/locales"
	"github.com/egoist/mygo/ui"
)

// Command is what a button that is not a tool does.
type Command uint8

const (
	CommandUndo Command = iota
	CommandRedo
	CommandCancel
	CommandSave
	CommandCopy
)

// ActionKind says which of the fields of an Action is set.
type ActionKind uint8

const (
	ActionTool ActionKind = iota
	ActionColor
	ActionWidth
	// ActionEmoji is one of the emoji the panel the emoji tool opens shows.
	ActionEmoji
	ActionCommand
)

// Action is what one button of the toolbar does.
type Action struct {
	Kind    ActionKind
	Tool    Kind
	Color   ui.Color
	Width   float32
	Emoji   string
	Command Command
}

// State is what the toolbar shows as chosen.
type State struct {
	Tool    Kind
	Color   ui.Color
	Width   float32
	CanUndo bool
	CanRedo bool
}

// Tool describes a tool of the toolbar.
type Tool struct {
	Kind Kind
	Icon *ui.SVG
	// Name is what the tool says to a screen reader or a tooltip.
	Name string
}

// Colors are the colors a shape can have, in the order the toolbar shows.
func Colors() []ui.Color {
	return []ui.Color{
		ui.RGB(240, 68, 56),
		ui.RGB(250, 146, 21),
		ui.RGB(247, 201, 52),
		ui.RGB(52, 199, 89),
		ui.RGB(48, 129, 240),
	}
}

// colorNames name the swatches of Colors, in its order.
func colorNames() []string {
	return []string{
		locales.ColorRed(),
		locales.ColorOrange(),
		locales.ColorYellow(),
		locales.ColorGreen(),
		locales.ColorBlue(),
	}
}

// widthNames name the buttons of Widths, in its order.
func widthNames() []string {
	return []string{locales.StrokeThin(), locales.StrokeMedium(), locales.StrokeThick()}
}

// Widths are the stroke widths the toolbar offers, in DIPs: the overlay
// scales them to the pixels of the capture.
func Widths() []float32 { return []float32{2, 4, 8} }

// Emojis are the emoji the panel of the emoji tool offers, in the order it
// shows them.
func Emojis() []string {
	return []string{
		"😀", "😃", "😄", "😁", "😆", "😅", "😂", "🤣", "😊", "😇", "🙂", "🙃",
		"😉", "😌", "😍", "🥰", "😘", "😋", "😎", "🤩", "🥳", "😏", "😢", "😭",
		"😤", "😡", "🥺", "😱", "🤔", "😴", "🤤", "😬", "🙄", "😲", "🤯", "😷",
		"👍", "👎", "👌", "✌️", "🙏", "👏", "🙌", "💪", "👋", "🤝", "👀", "💯",
		"🐶", "🐱", "🐭", "🐹", "🐰", "🦊", "🐻", "🐼", "🐨", "🐯", "🦁", "🐷",
		"🍎", "🍉", "🍓", "🍕", "🍔", "🍣", "🍰", "☕", "🔥", "⭐", "🎉", "❤️",
	}
}

// Tools are the tools the toolbar offers, in the order it shows them.
func Tools() []Tool {
	return []Tool{
		{KindRect, IconRect, locales.ToolRectangle()},
		{KindEllipse, IconEllipse, locales.ToolEllipse()},
		{KindArrow, IconArrow, locales.ToolArrow()},
		{KindPen, IconPen, locales.ToolPen()},
		{KindText, IconText, locales.ToolText()},
		{KindBadge, IconBadge, locales.ToolNumber()},
		{KindEmoji, IconEmoji, locales.ToolEmoji()},
		{KindMosaic, IconMosaic, locales.ToolMosaic()},
		{KindCrop, IconCrop, locales.ToolCrop()},
	}
}

// The toolbar's sizes, in DIPs.
const (
	barPad   = 8
	itemSize = 34
	swatch   = 22
	gap      = 3
	groupGap = 10
)

// Button is one control of the toolbar.
type Button struct {
	Rect   ui.Rect
	Action Action
	Icon   *ui.SVG
	Color  ui.Color
	// Text is drawn in place of an icon: the emoji a button stands for.
	Text     string
	Active   bool
	Disabled bool
	// Dot is the side of the dot a button that stands for a width draws, in
	// DIPs.
	Dot float32
	// Tip says what the button does, when the pointer rests on it.
	Tip string
}

// Bar is the toolbar, laid out for one frame of the window.
type Bar struct {
	Rect    ui.Rect
	Buttons []Button
}

// Layout places the toolbar for st over sel, in win: under the selection
// where it fits, over it where it does not, and along the bottom of the
// window where neither has room. A window too narrow for one row of buttons
// holds two.
func Layout(sel, win ui.Rect, st State) Bar {
	var buttons []Button
	x, y := float32(barPad), float32(barPad)
	content := max(win.W-2*barPad, itemSize)
	right := float32(0)
	add := func(b Button) {
		if x+b.Rect.W > barPad+content && x > barPad {
			x, y = barPad, y+itemSize+gap
		}
		b.Rect.X = x
		b.Rect.Y += y - barPad
		buttons = append(buttons, b)
		x = b.Rect.X + b.Rect.W + gap
		right = max(right, b.Rect.X+b.Rect.W)
	}
	for i, tool := range Tools() {
		if i > 0 {
			x += groupGap - gap
		}
		add(Button{
			Rect:   ui.Rect{X: x, Y: barPad, W: itemSize, H: itemSize},
			Action: Action{Kind: ActionTool, Tool: tool.Kind},
			Icon:   tool.Icon,
			Active: st.Tool == tool.Kind,
			Tip:    tool.Name,
		})
	}
	x += groupGap - gap
	for i, color := range Colors() {
		if i > 0 {
			x += groupGap - gap
		}
		add(Button{
			Rect:   ui.Rect{X: x, Y: barPad + (itemSize-swatch)/2, W: swatch, H: swatch},
			Action: Action{Kind: ActionColor, Color: color},
			Color:  color,
			Active: st.Color == color,
			Tip:    locales.ShapeColor(colorNames()[i]),
		})
	}
	x += groupGap - gap
	for i, width := range Widths() {
		add(Button{
			Rect:   ui.Rect{X: x, Y: barPad, W: itemSize, H: itemSize},
			Action: Action{Kind: ActionWidth, Width: width},
			Dot:    min(width*1.8, itemSize-8),
			Active: st.Width == width,
			Tip:    locales.StrokeTip(widthNames()[i], int(width)),
		})
	}
	x += groupGap - gap
	for _, cmd := range []struct {
		command  Command
		icon     *ui.SVG
		disabled bool
		tip      string
	}{
		{CommandUndo, IconUndo, !st.CanUndo, locales.CommandUndo()},
		{CommandRedo, IconRedo, !st.CanRedo, locales.CommandRedo()},
		{CommandCancel, IconCancel, false, locales.CommandCancel()},
		{CommandSave, IconSave, false, locales.CommandSave()},
		{CommandCopy, IconCopy, false, locales.CommandCopy()},
	} {
		add(Button{
			Rect:     ui.Rect{X: x, Y: barPad, W: itemSize, H: itemSize},
			Action:   Action{Kind: ActionCommand, Command: cmd.command},
			Icon:     cmd.icon,
			Disabled: cmd.disabled,
			Tip:      cmd.tip,
		})
	}

	at := placeToolbar(sel, win, min(right+barPad, win.W), y+itemSize+barPad, 8, 8)
	for i := range buttons {
		buttons[i].Rect.X += at.X
		buttons[i].Rect.Y += at.Y
	}
	return Bar{Rect: at, Buttons: buttons}
}

// pickerGap is how far the panel of emoji stands from the toolbar, in DIPs.
const pickerGap = 8

// layoutPickerCols bounds the columns of the panel of emoji, which is a
// block of them rather than a strip across the window.
const pickerCols = 12

// LayoutPicker places the panel of emoji the toolbar shows while the emoji
// tool is chosen: in as many columns as the window holds, over the toolbar
// where it fits and under it where it does not.
func LayoutPicker(bar Bar, win ui.Rect, current string) Bar {
	emojis := Emojis()
	cols := int((win.W - 2*barPad + gap) / (itemSize + gap))
	cols = min(max(cols, 1), len(emojis), pickerCols)
	rows := (len(emojis) + cols - 1) / cols
	w := float32(cols*itemSize + (cols-1)*gap + 2*barPad)
	h := float32(rows*itemSize + (rows-1)*gap + 2*barPad)
	x := min(max(bar.Rect.X, win.X+pickerGap), win.X+win.W-w-pickerGap)
	y := bar.Rect.Y - h - pickerGap
	if y < win.Y+pickerGap {
		y = bar.Rect.Y + bar.Rect.H + pickerGap
	}
	// A window too small for either side keeps the panel in it all the same.
	y = min(max(y, win.Y+pickerGap), max(win.Y+pickerGap, win.Y+win.H-h-pickerGap))
	buttons := make([]Button, len(emojis))
	for i, emoji := range emojis {
		row, col := i/cols, i%cols
		buttons[i] = Button{
			Rect: ui.Rect{
				X: x + barPad + float32(col)*(itemSize+gap),
				Y: y + barPad + float32(row)*(itemSize+gap),
				W: itemSize, H: itemSize,
			},
			Action: Action{Kind: ActionEmoji, Emoji: emoji},
			Text:   emoji,
			Active: emoji == current,
		}
	}
	return Bar{Rect: ui.Rect{X: x, Y: y, W: w, H: h}, Buttons: buttons}
}

// placeToolbar returns where a toolbar w×h goes, near sel in win.
func placeToolbar(sel, win ui.Rect, w, h, gapFromSelection, margin float32) ui.Rect {
	// Right aligned to the selection, and never off the left edge; a window
	// narrower than the toolbar keeps it in the window's top left corner.
	x := max(min(sel.X+sel.W-w, win.X+win.W-w-margin), win.X+margin)
	if y := sel.Y + sel.H + gapFromSelection; y+h <= win.Y+win.H-margin {
		return ui.Rect{X: x, Y: y, W: w, H: h}
	}
	if y := sel.Y - h - gapFromSelection; y >= win.Y+margin {
		return ui.Rect{X: x, Y: y, W: w, H: h}
	}
	return ui.Rect{X: x, Y: win.Y + win.H - h - margin, W: w, H: h}
}

// ForcedTheme is the look the toolbar takes whatever the desktop says: 1 for
// the light one, -1 for the dark one, 0 to follow the desktop.
var ForcedTheme int

// Look is the colors of the toolbar, by the theme the desktop asks for.
type Look struct {
	Face, Ink, Lit, Muted, Ring, Shadow ui.Color
}

// Looks are the toolbar in a light and in a dark desktop.
var (
	LightLook = Look{
		Face:   ui.RGBA(252, 252, 253, 0.96),
		Ink:    ui.RGB(28, 28, 32),
		Lit:    ui.RGBA(0, 0, 0, 0.07),
		Muted:  ui.RGBA(0, 0, 0, 0.28),
		Ring:   ui.RGB(255, 255, 255),
		Shadow: ui.RGBA(0, 0, 0, 0.18),
	}
	DarkLook = Look{
		Face:   ui.RGBA(24, 24, 28, 0.94),
		Ink:    ui.RGB(255, 255, 255),
		Lit:    ui.RGBA(255, 255, 255, 0.14),
		Muted:  ui.RGBA(255, 255, 255, 0.30),
		Ring:   ui.RGB(255, 255, 255),
		Shadow: ui.RGBA(0, 0, 0, 0.35),
	}
)

// LookFor picks the toolbar's colors from the theme, or from what
// ForcedTheme asks for.
func LookFor(t *ui.Theme) Look {
	dark := t != nil && t.Dark
	if ForcedTheme != 0 {
		dark = ForcedTheme < 0
	}
	if dark {
		return DarkLook
	}
	return LightLook
}

// TipDelay is how long the pointer rests on a button before its tip shows.
const TipDelay = 400 * time.Millisecond

// DrawTip names a button: a line under anchor, or over it where the window
// has no room below, kept inside box.
func DrawTip(p *ui.Painter, anchor ui.Rect, text string, box ui.Rect) {
	const size, padX, padY = float32(12), float32(8), float32(6)
	w, h := p.MeasureText(0, ui.Span{Text: text, Size: size})
	x := min(max(anchor.X+(anchor.W-w)/2, box.X+4), box.X+box.W-w-4)
	y := anchor.Y + anchor.H + 8
	if y+h+2*padY > box.Y+box.H-4 {
		y = anchor.Y - h - 2*padY - 8
	}
	p.Fill(ui.Rect{X: x - padX, Y: y, W: w + 2*padX, H: h + 2*padY}, ui.RGBA(24, 24, 28, 0.96), 6)
	p.Text(x, y+padY, text, size, ui.RGB(255, 255, 255))
}

// Move shifts the toolbar by dx and dy.
func (b *Bar) Move(dx, dy float32) {
	b.Rect.X += dx
	b.Rect.Y += dy
	for i := range b.Buttons {
		b.Buttons[i].Rect.X += dx
		b.Buttons[i].Rect.Y += dy
	}
}

// Hit returns the button of b at p.
func (b Bar) Hit(p ui.Point) (Button, bool) {
	for _, button := range b.Buttons {
		if button.Rect.Contains(p.X, p.Y) {
			return button, true
		}
	}
	return Button{}, false
}

// Button returns the button that runs cmd.
func (b Bar) Button(cmd Command) (Button, bool) {
	for _, button := range b.Buttons {
		if button.Action.Kind == ActionCommand && button.Action.Command == cmd {
			return button, true
		}
	}
	return Button{}, false
}

// Tool returns the button that picks k.
func (b Bar) Tool(k Kind) (Button, bool) {
	for _, button := range b.Buttons {
		if button.Action.Kind == ActionTool && button.Action.Tool == k {
			return button, true
		}
	}
	return Button{}, false
}

// Holds reports whether p is inside the bar.
func (b Bar) Holds(p ui.Point) bool { return b.Rect.Contains(p.X, p.Y) }

// Draw paints the toolbar, lighting the button under the pointer.
func (b Bar) Draw(p *ui.Painter, pointer ui.Point, hovering bool) {
	look := LookFor(p.Theme())
	p.Shadow(b.Rect, 12, 0, 6, 16, 0, look.Shadow)
	p.Fill(b.Rect, look.Face, 12)
	for _, button := range b.Buttons {
		lit := hovering && button.Rect.Contains(pointer.X, pointer.Y)
		// A swatch marks what is chosen with its ring, not a panel behind it.
		swatch := button.Color.A > 0
		switch {
		case button.Active && !swatch:
			p.Fill(button.Rect, ui.RGB(48, 129, 240), 7)
		case lit && !button.Active && !swatch:
			p.Fill(button.Rect, look.Lit, 7)
		}
		switch {
		case button.Icon != nil:
			tint := look.Ink
			if button.Disabled {
				tint = look.Muted
			}
			const inset = float32(7)
			p.Icon(button.Icon, ui.Rect{
				X: button.Rect.X + inset, Y: button.Rect.Y + inset,
				W: button.Rect.W - 2*inset, H: button.Rect.H - 2*inset,
			}, tint)
		case swatch:
			if button.Active || lit {
				p.Fill(button.Rect, look.Ring, button.Rect.W/2)
			}
			const inner = float32(3)
			p.Fill(ui.Rect{
				X: button.Rect.X + inner, Y: button.Rect.Y + inner,
				W: button.Rect.W - 2*inner, H: button.Rect.H - 2*inner,
			}, button.Color, button.Rect.W/2)
		case button.Text != "":
			// An emoji draws its glyph as the button's face, sized to the
			// cell it sits in.
			size := button.Rect.H * 0.58
			w, h := p.MeasureText(0, ui.Span{Text: button.Text, Size: size})
			p.Text(button.Rect.X+(button.Rect.W-w)/2, button.Rect.Y+(button.Rect.H-h)/2, button.Text, size, look.Ink)
		default:
			d := button.Dot
			p.Fill(ui.Rect{
				X: button.Rect.X + (button.Rect.W-d)/2,
				Y: button.Rect.Y + (button.Rect.H-d)/2,
				W: d, H: d,
			}, look.Ink, d/2)
		}
	}
}
