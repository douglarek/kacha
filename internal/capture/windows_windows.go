//go:build windows

package capture

import (
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// The calls that list the windows of the desktop, and the numbers they take.
const (
	dwmwaExtendedFrameBounds = 9
	dwmwaCloaked             = 14
	monitorDefaultToNearest  = 2
	effectiveDPI             = 0
	queryLimitedInformation  = 0x1000
	// smallest is the side a window needs, in DIPs, to be one a click could
	// pick.
	smallest = 40
)

var (
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	shcore   = syscall.NewLazyDLL("shcore.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procEnumWindows              = user32.NewProc("EnumWindows")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procIsIconic                 = user32.NewProc("IsIconic")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procMonitorFromWindow        = user32.NewProc("MonitorFromWindow")
	procGetDpiForMonitor         = shcore.NewProc("GetDpiForMonitor")
	procDwmGetWindowAttribute    = dwmapi.NewProc("DwmGetWindowAttribute")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImage    = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle              = kernel32.NewProc("CloseHandle")

	// windowsCallback, and what it collects, are one at a time, as
	// EnumWindows runs on the thread that calls it.
	windowsOnce     sync.Once
	windowsCallback uintptr
	windowsMu       sync.Mutex
	collected       []Window
)

// winRect is RECT.
type winRect struct{ Left, Top, Right, Bottom int32 }

// Windows lists the windows of the desktop a click could pick, topmost
// first, in the DIPs of the display they are on.
func Windows() ([]Window, error) {
	windowsOnce.Do(func() {
		windowsCallback = syscall.NewCallback(collectWindow)
	})
	windowsMu.Lock()
	defer windowsMu.Unlock()
	collected = nil
	procEnumWindows.Call(windowsCallback, 0)
	return collected, nil
}

// collectWindow takes the windows EnumWindows hands over, and keeps those a
// click could pick.
func collectWindow(hwnd, _ uintptr) uintptr {
	if visible, _, _ := procIsWindowVisible.Call(hwnd); visible == 0 {
		return 1
	}
	if iconic, _, _ := procIsIconic.Call(hwnd); iconic != 0 {
		return 1
	}
	var cloaked uint32
	procDwmGetWindowAttribute.Call(hwnd, dwmwaCloaked, uintptr(unsafe.Pointer(&cloaked)), unsafe.Sizeof(cloaked))
	if cloaked != 0 {
		return 1
	}
	rect, ok := frameOf(hwnd)
	if !ok {
		return 1
	}
	scale := monitorScale(hwnd)
	dip := func(v int32) float64 { return float64(v) / scale }
	box := image.Rect(
		int(math.Round(dip(rect.Left))),
		int(math.Round(dip(rect.Top))),
		int(math.Round(dip(rect.Right))),
		int(math.Round(dip(rect.Bottom))),
	)
	if box.Dx() < smallest || box.Dy() < smallest {
		return 1
	}
	app := appOf(hwnd)
	if app == "" {
		return 1
	}
	collected = append(collected, Window{Rect: box, App: app, Title: titleOf(hwnd)})
	return 1
}

// frameOf returns the frame a window shows, without the shadow around it.
func frameOf(hwnd uintptr) (winRect, bool) {
	var rect winRect
	r, _, _ := procDwmGetWindowAttribute.Call(hwnd, dwmwaExtendedFrameBounds,
		uintptr(unsafe.Pointer(&rect)), unsafe.Sizeof(rect))
	if r != 0 {
		if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
			return rect, false
		}
	}
	return rect, rect.Right > rect.Left && rect.Bottom > rect.Top
}

// monitorScale returns the pixels of the display the window is on, per DIP.
func monitorScale(hwnd uintptr) float64 {
	monitor, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if monitor == 0 {
		return 1
	}
	var x, y uint32
	if r, _, _ := procGetDpiForMonitor.Call(monitor, effectiveDPI,
		uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y))); r != 0 || x == 0 {
		return 1
	}
	return float64(x) / 96
}

// appOf returns the program of the window, as its file name without the
// suffix, or "" for a window of this app.
func appOf(hwnd uintptr) string {
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 || pid == uint32(os.Getpid()) {
		return ""
	}
	process, _, _ := procOpenProcess.Call(queryLimitedInformation, 0, uintptr(pid))
	if process == 0 {
		return ""
	}
	defer procCloseHandle.Call(process)
	buf := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buf))
	ok, _, _ := procQueryFullProcessImage.Call(process, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return ""
	}
	name := filepath.Base(syscall.UTF16ToString(buf[:size]))
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// titleOf returns the title of the window.
func titleOf(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), length+1)
	return syscall.UTF16ToString(buf)
}
