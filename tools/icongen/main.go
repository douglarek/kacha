// Command icongen draws the icons of the app from their sources.
//
//	go run ./tools/icongen
package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"

	"github.com/egoist/mygo/ui"
)

func main() {
	for _, icon := range []struct {
		name string
		cut  cutter
	}{
		{"icon", round}, {"icon-dev", round},
	} {
		if err := draw("resources/"+icon.name+".svg", "resources/"+icon.name+".png", icon.cut); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

// cutter takes the white of a frame away, as a drawing on white leaves it
// where nothing was drawn.
type cutter func(img *image.RGBA) image.Image

// round cuts the white outside the squircle of the app icon, whose source
// draws it with a radius of 230 of 1024.
func round(img *image.RGBA) image.Image {
	b := img.Bounds()
	size := float64(b.Dx())
	radius := 230 * size / 1024
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if !inSquircle(float64(x)+0.5, float64(y)+0.5, size, radius) {
				continue
			}
			out.Set(x, y, img.At(x, y))
		}
	}
	return out
}

// inSquircle tells whether a point of a size square is inside its squircle
// of that radius.
func inSquircle(x, y, size, radius float64) bool {
	cx, cy := x-(size/2), y-(size/2)
	half := size/2 - radius
	dx, dy := math.Abs(cx)-half, math.Abs(cy)-half
	if dx <= 0 || dy <= 0 {
		return math.Abs(cx) <= size/2 && math.Abs(cy) <= size/2
	}
	return dx*dx+dy*dy <= radius*radius
}

// draw renders the SVG at src into the PNG at dst, as large as the SVG says.
func draw(src, dst string, cut cutter) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	svg, err := ui.ParseSVG(data)
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	width, height := 1024, 1024
	if w, h := svg.Size(); w > 0 && h > 0 {
		width, height = int(w+0.5), int(h+0.5)
	}
	frame := ui.Render(func(c *ui.Context) {
		// Not filled: what the SVG leaves empty stays transparent.
		ui.Box(c).Grow(1).Draw(func(p *ui.Painter, r ui.Rect) {
			p.Image(svg, r, ui.FillBox)
		})
	}, width, height, 1)
	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	if err := png.Encode(file, cut(frame)); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
