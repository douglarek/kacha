//go:build windows

package capture

import (
	"context"
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

// The calls of user32 and gdi32 a capture makes.
var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procCreateBitmap       = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procBitBlt             = gdi32.NewProc("BitBlt")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procGetDIBits          = gdi32.NewProc("GetDIBits")
)

// The metrics and flags of a capture of the virtual screen.
const (
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	srcCopy       = 0x00CC0020
	dibRGB        = 0
	dibHeaderSize = 40
)

// bitmapInfoHeader is BITMAPINFOHEADER, the header of the bitmap a capture
// reads.
type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// desktopBackend captures the screen through GDI, which every Windows that
// runs a window of mygo has.
type desktopBackend struct{}

// Detect returns the backend Windows captures with.
func Detect() (Backend, error) { return desktopBackend{}, nil }

// Name identifies the backend.
func (desktopBackend) Name() string { return "Windows GDI" }

// Capture returns the virtual screen, the displays together, in the pixels
// of a desktop that knows its DPI.
func (desktopBackend) Capture(ctx context.Context) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	x, _, _ := procGetSystemMetrics.Call(smXVirtualScreen)
	y, _, _ := procGetSystemMetrics.Call(smYVirtualScreen)
	width, _, _ := procGetSystemMetrics.Call(smCXVirtualScreen)
	height, _, _ := procGetSystemMetrics.Call(smCYVirtualScreen)
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("%w: the desktop reports no screen", ErrUnsupported)
	}

	screen, _, _ := procGetDC.Call(0)
	if screen == 0 {
		return nil, fmt.Errorf("capture: GetDC: %w", syscall.GetLastError())
	}
	defer procReleaseDC.Call(0, screen)

	memory, _, _ := procCreateCompatibleDC.Call(screen)
	if memory == 0 {
		return nil, fmt.Errorf("capture: CreateCompatibleDC: %w", syscall.GetLastError())
	}
	defer procDeleteDC.Call(memory)

	bitmap, _, _ := procCreateBitmap.Call(screen, width, height)
	if bitmap == 0 {
		return nil, fmt.Errorf("capture: CreateCompatibleBitmap: %w", syscall.GetLastError())
	}
	defer procDeleteObject.Call(bitmap)

	previous, _, _ := procSelectObject.Call(memory, bitmap)
	defer procSelectObject.Call(memory, previous)

	if r, _, err := procBitBlt.Call(memory, 0, 0, width, height, screen, x, y, srcCopy); r == 0 {
		return nil, fmt.Errorf("capture: BitBlt: %w", err)
	}

	// A top-down DIB of 32 bits: what BitBlt wrote, as rows of BGRA.
	header := bitmapInfoHeader{
		Size:     dibHeaderSize,
		Width:    int32(width),
		Height:   -int32(height),
		Planes:   1,
		BitCount: 32,
	}
	pixels := make([]byte, int(width*height*4))
	if r, _, err := procGetDIBits.Call(memory, bitmap, 0, height,
		uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&header)), dibRGB); r == 0 {
		return nil, fmt.Errorf("capture: GetDIBits: %w", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for i, j := 0, 0; i+3 < len(pixels); i, j = i+4, j+4 {
		img.Pix[j] = pixels[i+2]
		img.Pix[j+1] = pixels[i+1]
		img.Pix[j+2] = pixels[i]
		img.Pix[j+3] = 0xFF
	}
	return img, nil
}
