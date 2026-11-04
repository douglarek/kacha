//go:build windows

package shot

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// The calls a copy to the clipboard makes.
var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procEmptyClipboard             = user32.NewProc("EmptyClipboard")
	procSetClipboardData           = user32.NewProc("SetClipboardData")
	procRegisterClipboardFormatW   = user32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc                = kernel32.NewProc("GlobalAlloc")
	procGlobalLock                 = kernel32.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32.NewProc("GlobalUnlock")
	procGlobalFree                 = kernel32.NewProc("GlobalFree")
	procRtlMoveMemory              = kernel32.NewProc("RtlMoveMemory")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
)

// The formats a capture goes on the clipboard in: the bitmap every Windows
// app pastes, PNG for the ones that keep its alpha, and a file, which is
// what Explorer pastes.
const (
	cfDIB        = 8
	cfHDROP      = 15
	gmemMoveable = 0x0002
	pngFormat    = "PNG"
)

// copyPNG puts data on the clipboard for good, as the app ends before the
// next paste: nothing is left for it to serve.
func copyPNG(data []byte) error {
	// The clipboard belongs to the thread that opened it, and this goroutine
	// may move to another one mid-way.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("kacha: decode for the clipboard: %w", err)
	}
	// Another app may hold the clipboard for a moment.
	var opened uintptr
	for range 10 {
		if opened, _, _ = procOpenClipboard.Call(0); opened != 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if opened == 0 {
		return fmt.Errorf("kacha: open the clipboard: %w", syscall.GetLastError())
	}
	closed := false
	defer func() {
		if !closed {
			procCloseClipboard.Call()
		}
	}()

	procEmptyClipboard.Call()
	if err := setClipboard(cfDIB, dib(img)); err != nil {
		return err
	}
	pngID := registerFormat(pngFormat)
	if pngID != 0 {
		if err := setClipboard(pngID, data); err != nil {
			return err
		}
	}
	// Explorer pastes files, not pictures: a copy is a file too.
	if path, err := writeTemp(data); err == nil {
		if err := setClipboard(cfHDROP, dropFiles(path)); err != nil {
			return err
		}
	}
	procCloseClipboard.Call()
	closed = true

	if r, _, _ := procIsClipboardFormatAvailable.Call(cfDIB); r == 0 {
		return errors.New("kacha: the clipboard kept no bitmap")
	}
	if pngID != 0 {
		if r, _, _ := procIsClipboardFormatAvailable.Call(pngID); r == 0 {
			return errors.New("kacha: the clipboard kept no PNG")
		}
	}
	return nil
}

// writeTemp writes the capture to a file that outlives the app, and takes
// the files a day older than this one away.
func writeTemp(data []byte) (string, error) {
	dir := filepath.Join(os.TempDir(), "kacha")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	old, err := filepath.Glob(filepath.Join(dir, "kacha-*.png"))
	if err == nil {
		for _, path := range old {
			if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) > 24*time.Hour {
				os.Remove(path)
			}
		}
	}
	path := filepath.Join(dir, time.Now().Format("kacha-2006-01-02-15-04-05.000")+".png")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// dropFiles writes paths as CF_HDROP: a DROPFILES header and the names in
// UTF-16, each ended by a zero, the list by two.
func dropFiles(paths ...string) []byte {
	out := make([]byte, 20, 20+64*len(paths))
	le := binary.LittleEndian
	le.PutUint32(out[0:], 20) // DROPFILES.pFiles, the size of the header
	le.PutUint32(out[16:], 1) // fWide
	for _, path := range paths {
		utf16, err := syscall.UTF16FromString(path)
		if err != nil {
			continue
		}
		for _, unit := range utf16 {
			out = le.AppendUint16(out, unit)
		}
	}
	return append(out, 0, 0)
}

// setClipboard hands data to the clipboard in format, which owns it from
// then on.
func setClipboard(format uintptr, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	memory, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
	if memory == 0 {
		return fmt.Errorf("kacha: allocate for the clipboard: %w", syscall.GetLastError())
	}
	addr, _, _ := procGlobalLock.Call(memory)
	if addr == 0 {
		procGlobalFree.Call(memory)
		return fmt.Errorf("kacha: lock the clipboard memory: %w", syscall.GetLastError())
	}
	procRtlMoveMemory.Call(addr, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	procGlobalUnlock.Call(memory)
	if r, _, err := procSetClipboardData.Call(format, memory); r == 0 {
		procGlobalFree.Call(memory)
		return fmt.Errorf("kacha: put the capture on the clipboard: %w", err)
	}
	return nil
}

// registerFormat returns the clipboard format of name, registering it the
// first time.
func registerFormat(name string) uintptr {
	utf16, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0
	}
	format, _, _ := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(utf16)))
	return format
}

// dib writes img as the device independent bitmap of the clipboard: a
// BITMAPINFOHEADER and rows of BGRA, the last row first, which is what
// every Windows app that pastes a picture reads.
func dib(img image.Image) []byte {
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	const headerSize = 40
	out := make([]byte, headerSize+width*height*4)
	le := binary.LittleEndian
	le.PutUint32(out[0:], headerSize)
	le.PutUint32(out[4:], uint32(int32(width)))
	le.PutUint32(out[8:], uint32(int32(height)))
	le.PutUint16(out[12:], 1)
	le.PutUint16(out[14:], 32)
	le.PutUint32(out[16:], 0) // BI_RGB
	le.PutUint32(out[20:], uint32(width*height*4))
	for y := range height {
		row := out[headerSize+(height-1-y)*width*4:]
		for x := range width {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			px := row[x*4:]
			px[0] = uint8(bl >> 8)
			px[1] = uint8(g >> 8)
			px[2] = uint8(r >> 8)
			px[3] = uint8(a >> 8)
		}
	}
	return out
}
