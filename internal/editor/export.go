package editor

import (
	"image"

	"github.com/egoist/mygo/ui"
)

// Render draws img with the board's shapes over it, in the pixels of the
// capture: what the selection window shows, at the size it was captured.
func Render(img image.Image, board *Board, mosaic *Mosaic) *image.RGBA {
	bounds := img.Bounds()
	bitmap := ui.NewBitmap(img)
	return ui.Render(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, box ui.Rect) {
			p.Image(bitmap, box, ui.FillBox)
			Paint(p, board.Shapes(), mosaic, 1, ui.Rect{})
		})
	}, bounds.Dx(), bounds.Dy(), 1)
}
