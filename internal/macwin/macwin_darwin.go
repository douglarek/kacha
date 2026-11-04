//go:build darwin

// Package macwin drives the windows of macOS from code mygo does not cover.
package macwin

import (
	"runtime"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
)

// statusLevel is NSStatusWindowLevel: above the menu bar, which macOS would
// otherwise keep an ordinary window from covering.
const statusLevel = 25

var (
	objcOnce    sync.Once
	objcErr     error
	objcClass   func(name *byte) uintptr
	objcSelName func(name *byte) uintptr
	objcSend2   func(obj, sel uintptr) uintptr
	objcSend3   func(obj, sel uintptr, arg uint64) uintptr

	classOnce sync.Once
	classErr  error
	// keyableIMP is the C function that answers yes to canBecomeKeyWindow,
	// kept alive while it runs.
	keyableIMP uintptr
)

// CoverMenuBar lifts the window above the menu bar, so that it covers the
// whole display rather than the work area below it, and keeps the keys, so
// that Escape leaves a capture started from anywhere. AppKit runs on the
// main thread, and so does this.
func CoverMenuBar(w *mygo.Window) {
	objcOnce.Do(loadObjC)
	if objcErr != nil || w == nil {
		return
	}
	mygo.RunOnMain(func() {
		handle := w.NativeHandle()
		if handle == 0 {
			return
		}
		// A borderless window covers the menu bar but takes no keys, as
		// NSWindow answers no to canBecomeKeyWindow; mygo's windows answer
		// yes from here on.
		AllowKeys()
		style := cstr("setStyleMask:")
		objcSend3(handle, objcSelName(&style[0]), 0)
		runtime.KeepAlive(style)
		level := cstr("setLevel:")
		objcSend3(handle, objcSelName(&level[0]), statusLevel)
		runtime.KeepAlive(level)

		// The app may be in the background when a shortcut starts a
		// capture, where a window takes no keys until the app comes first.
		appName := cstr("NSApplication")
		appClass := objcClass(&appName[0])
		runtime.KeepAlive(appName)
		if appClass != 0 {
			shared := cstr("sharedApplication")
			app := objcSend2(appClass, objcSelName(&shared[0]))
			runtime.KeepAlive(shared)
			if app != 0 {
				activate := cstr("activateIgnoringOtherApps:")
				objcSend3(app, objcSelName(&activate[0]), 1)
				runtime.KeepAlive(activate)
			}
		}
		front := cstr("makeKeyAndOrderFront:")
		objcSend3(handle, objcSelName(&front[0]), 0)
		runtime.KeepAlive(front)
	})
}

// Activate brings the app forward and makes w its key window, so that it
// takes the keys a click on it would.
func Activate(w *mygo.Window) {
	objcOnce.Do(loadObjC)
	if objcErr != nil || w == nil {
		return
	}
	mygo.RunOnMain(func() {
		handle := w.NativeHandle()
		if handle == 0 {
			return
		}
		AllowKeys()
		activateApp()
		front := cstr("makeKeyAndOrderFront:")
		objcSend3(handle, objcSelName(&front[0]), 0)
		runtime.KeepAlive(front)
	})
}

// activateApp brings the app to the front, which a window of a background
// app needs before it takes keys.
func activateApp() {
	appName := cstr("NSApplication")
	appClass := objcClass(&appName[0])
	runtime.KeepAlive(appName)
	if appClass == 0 {
		return
	}
	shared := cstr("sharedApplication")
	app := objcSend2(appClass, objcSelName(&shared[0]))
	runtime.KeepAlive(shared)
	if app == 0 {
		return
	}
	activate := cstr("activateIgnoringOtherApps:")
	objcSend3(app, objcSelName(&activate[0]), 1)
	runtime.KeepAlive(activate)
}

// AllowKeys makes the classes of mygo's windows answer yes to
// canBecomeKeyWindow, which a borderless window answers no to, so that the
// overlay takes the keys it needs: Escape, Enter, undo and text.
func AllowKeys() {
	classOnce.Do(loadKeyWindows)
}

// loadKeyWindows adds canBecomeKeyWindow to the windows of mygo, once.
func loadKeyWindows() {
	lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		classErr = err
		return
	}
	var addMethod func(cls, sel, imp uintptr, types *byte) bool
	var getClass func(name *byte) uintptr
	var selName func(name *byte) uintptr
	purego.RegisterLibFunc(&addMethod, lib, "class_addMethod")
	purego.RegisterLibFunc(&getClass, lib, "objc_getClass")
	purego.RegisterLibFunc(&selName, lib, "sel_registerName")

	keyableIMP = purego.NewCallback(func(_, _ uintptr) uintptr { return 1 })
	types := cstr("B@:")
	for _, className := range []string{"MyGoWindow", "MyGoPanel"} {
		name := cstr(className)
		cls := getClass(&name[0])
		runtime.KeepAlive(name)
		if cls == 0 {
			continue
		}
		for _, selector := range []string{"canBecomeKeyWindow", "canBecomeMainWindow"} {
			sel := cstr(selector)
			addMethod(cls, selName(&sel[0]), keyableIMP, &types[0])
			runtime.KeepAlive(sel)
		}
	}
	runtime.KeepAlive(types)
}

// cstr returns s as a C string, which the caller keeps alive.
func cstr(s string) []byte { return append([]byte(s), 0) }

// loadObjC opens libobjc for the window calls.
func loadObjC() {
	lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		objcErr = err
		return
	}
	purego.RegisterLibFunc(&objcClass, lib, "objc_getClass")
	purego.RegisterLibFunc(&objcSelName, lib, "sel_registerName")
	purego.RegisterLibFunc(&objcSend2, lib, "objc_msgSend")
	purego.RegisterLibFunc(&objcSend3, lib, "objc_msgSend")
}
