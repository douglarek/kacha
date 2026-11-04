//go:build darwin

package capture

import (
	"image"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The windows of CGWindowListCopyWindowInfo.
const (
	cgWindowListOnScreenOnly   = 1 << 0
	cgWindowListExcludeDesktop = 1 << 4
	cfStringEncodingUTF8       = 0x08000100
	cfNumberFloat64Type        = 13

	keyBounds     = "kCGWindowBounds"
	keyOwnerName  = "kCGWindowOwnerName"
	keyWindowName = "kCGWindowName"
	keyLayer      = "kCGWindowLayer"
	keyAlpha      = "kCGWindowAlpha"
)

// cgRect is a CGRect.
type cgRect struct{ X, Y, W, H float64 }

var (
	cgWindowListCopyWindowInfo func(option, relativeTo uint32) uintptr
	cgRectFromDictionary       func(dict uintptr, rect *cgRect) bool
	cfArrayGetCount            func(array uintptr) int64
	cfArrayGetValueAtIndex     func(array uintptr, index int64) uintptr
	cfDictionaryGetValue       func(dict, key uintptr) uintptr
	cfStringCreateWithCString  func(alloc uintptr, s *byte, encoding uint32) uintptr
	cfStringGetCString         func(s uintptr, buf *byte, size int64, encoding uint32) bool
	cfNumberGetValue           func(number uintptr, kind int32, value unsafe.Pointer) bool
	cfRelease                  func(p uintptr)

	windowsOnce sync.Once
	windowsErr  error
)

// Windows returns the windows on screen, topmost first, in screen DIPs.
// Their geometry and owners need no permission; their titles need the one
// that captures the screen.
func Windows() ([]Window, error) {
	windowsOnce.Do(loadWindows)
	if windowsErr != nil {
		return nil, windowsErr
	}
	list := cgWindowListCopyWindowInfo(cgWindowListOnScreenOnly|cgWindowListExcludeDesktop, 0)
	if list == 0 {
		return nil, nil
	}
	defer cfRelease(list)

	var out []Window
	for i, n := int64(0), cfArrayGetCount(list); i < n; i++ {
		dict := cfArrayGetValueAtIndex(list, i)
		if dict == 0 {
			continue
		}
		var bounds cgRect
		if v := dictionaryValue(dict, keyBounds); v == 0 || !cgRectFromDictionary(v, &bounds) {
			continue
		}
		if numberValue(dict, keyLayer, 0) != 0 || numberValue(dict, keyAlpha, 1) < 0.05 {
			continue
		}
		if bounds.W < 40 || bounds.H < 40 {
			continue
		}
		out = append(out, Window{
			Rect: image.Rect(
				int(bounds.X), int(bounds.Y),
				int(bounds.X+bounds.W), int(bounds.Y+bounds.H),
			),
			App:   stringValue(dict, keyOwnerName),
			Title: stringValue(dict, keyWindowName),
		})
	}
	return out, nil
}

// loadWindows opens the frameworks Windows reads.
func loadWindows() {
	gfx, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		windowsErr = err
		return
	}
	purego.RegisterLibFunc(&cgWindowListCopyWindowInfo, gfx, "CGWindowListCopyWindowInfo")
	purego.RegisterLibFunc(&cgRectFromDictionary, gfx, "CGRectMakeWithDictionaryRepresentation")
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		windowsErr = err
		return
	}
	purego.RegisterLibFunc(&cfArrayGetCount, cf, "CFArrayGetCount")
	purego.RegisterLibFunc(&cfArrayGetValueAtIndex, cf, "CFArrayGetValueAtIndex")
	purego.RegisterLibFunc(&cfDictionaryGetValue, cf, "CFDictionaryGetValue")
	purego.RegisterLibFunc(&cfStringCreateWithCString, cf, "CFStringCreateWithCString")
	purego.RegisterLibFunc(&cfStringGetCString, cf, "CFStringGetCString")
	purego.RegisterLibFunc(&cfNumberGetValue, cf, "CFNumberGetValue")
	purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")
}

// dictionaryValue returns what a key of a dictionary holds, 0 when it has
// no such key.
func dictionaryValue(dict uintptr, key string) uintptr {
	k := makeCFString(key)
	if k == 0 {
		return 0
	}
	defer cfRelease(k)
	return cfDictionaryGetValue(dict, k)
}

// stringValue returns the string a key of a dictionary holds.
func stringValue(dict uintptr, key string) string {
	v := dictionaryValue(dict, key)
	if v == 0 {
		return ""
	}
	buf := make([]byte, 1024)
	if !cfStringGetCString(v, &buf[0], int64(len(buf)), cfStringEncodingUTF8) {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

// numberValue returns the number a key of a dictionary holds.
func numberValue(dict uintptr, key string, fallback float64) float64 {
	v := dictionaryValue(dict, key)
	if v == 0 {
		return fallback
	}
	var out float64
	if !cfNumberGetValue(v, cfNumberFloat64Type, unsafe.Pointer(&out)) {
		return fallback
	}
	return out
}

// makeCFString returns a CFString for s, which the caller releases.
func makeCFString(s string) uintptr {
	buf := append([]byte(s), 0)
	return cfStringCreateWithCString(0, &buf[0], cfStringEncodingUTF8)
}
