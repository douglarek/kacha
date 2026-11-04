//go:build darwin

package settings

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// keyNames name the keys of a shortcut that type no character, by the key
// code macOS gives them.
var keyNames = map[uint16]string{
	53:  "Escape",
	49:  "Space",
	122: "F1", 120: "F2", 99: "F3", 118: "F4", 96: "F5", 97: "F6",
	98: "F7", 100: "F8", 101: "F9", 109: "F10", 103: "F11", 111: "F12",
}

// The modifiers of a shortcut, as NSEvent's modifierFlags holds them.
const (
	flagShift   = 1 << 17
	flagControl = 1 << 18
	flagOption  = 1 << 19
	flagCommand = 1 << 20
)

// maskKeyDown is NSEventMaskKeyDown.
const maskKeyDown = 1 << 10

var (
	keysOnce sync.Once
	keysErr  error
	keysMsg  uintptr
	keysSel  func(name *byte) uintptr
	keysCls  func(name *byte) uintptr

	// The block handed to AppKit, and the Go function behind it, stay for
	// the life of the app: a monitor may call them at any time.
	blockPtr  *keyBlock
	blockDesc *keyBlockDescriptor
	blockFn   uintptr

	monitor   uintptr
	monitorFn func(combo string, escape bool) bool
)

// keyBlock is the layout of a block, which holds nothing else when global.
type keyBlock struct {
	isa        uintptr
	flags      uintptr
	invoke     uintptr
	descriptor uintptr
}

// keyBlockDescriptor describes a block to the runtime.
type keyBlockDescriptor struct {
	reserved uintptr
	size     uintptr
}

// WatchKeys calls fn with every key pressed while the app is in front, as
// an accelerator such as "CmdOrCtrl+Shift+S", with escape told for Escape,
// until the returned func stops it. A true from fn swallows the key.
func WatchKeys(fn func(combo string, escape bool) bool) func() {
	keysOnce.Do(loadKeys)
	if keysErr != nil {
		Logf("settings: keys: %v", keysErr)
		return func() {}
	}
	if monitor != 0 {
		removeMonitor()
	}
	monitorFn = fn
	monitor = keysCall(keysCls(cstrPtr("NSEvent")), keysSel(cstrPtr("addLocalMonitorForEventsMatchingMask:handler:")),
		maskKeyDown, uintptr(unsafe.Pointer(blockPtr)))
	return func() {
		removeMonitor()
		monitorFn = nil
	}
}

// removeMonitor takes the monitor of the keys away, if there is one.
func removeMonitor() {
	if monitor == 0 {
		return
	}
	keysCall(keysCls(cstrPtr("NSEvent")), keysSel(cstrPtr("removeMonitor:")), monitor)
	monitor = 0
}

// loadKeys opens the Objective-C runtime for the monitor of the keys, once.
func loadKeys() {
	lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		keysErr = err
		return
	}
	if keysMsg, err = purego.Dlsym(lib, "objc_msgSend"); err != nil {
		keysErr = err
		return
	}
	purego.RegisterLibFunc(&keysCls, lib, "objc_getClass")
	purego.RegisterLibFunc(&keysSel, lib, "sel_registerName")

	blockFn = purego.NewCallback(handleKey)
	blockDesc = &keyBlockDescriptor{size: unsafe.Sizeof(keyBlock{})}
	blockPtr = &keyBlock{
		isa:        keysCls(cstrPtr("__NSGlobalBlock__")),
		invoke:     blockFn,
		descriptor: uintptr(unsafe.Pointer(blockDesc)),
	}
}

// keysCall sends a message to an object.
func keysCall(obj, sel uintptr, args ...uintptr) uintptr {
	r, _, _ := purego.SyscallN(keysMsg, append([]uintptr{obj, sel}, args...)...)
	return r
}

// handleKey reads a key event, says what shortcut came in, and keeps the
// event from the app when that shortcut is taken.
func handleKey(_, event uintptr) uintptr {
	fn := monitorFn
	if fn == nil {
		return event
	}
	keyCode := uint16(keysCall(event, keysSel(cstrPtr("keyCode"))) & 0xffff)
	flags := keysCall(event, keysSel(cstrPtr("modifierFlags")))
	chars := keysCall(event, keysSel(cstrPtr("charactersIgnoringModifiers")))
	combo := accelerator(flags, keyCode, chars)
	if combo == "" {
		return event
	}
	if fn(combo, keyCode == 53) {
		return 0
	}
	return event
}

// accelerator writes a key event as the accelerator of mygo, such as
// "CmdOrCtrl+Shift+S", or "" for a key no shortcut may end with.
func accelerator(flags uintptr, keyCode uint16, chars uintptr) string {
	key := keyNames[keyCode]
	if key == "" && chars != 0 {
		if keysCall(chars, keysSel(cstrPtr("length"))) > 0 {
			r := rune(keysCall(chars, keysSel(cstrPtr("characterAtIndex:")), 0) & 0xffff)
			key = strings.ToUpper(string(r))
		}
	}
	if key == "" {
		return ""
	}
	var mods []string
	if flags&flagCommand != 0 {
		mods = append(mods, "CmdOrCtrl")
	}
	if flags&flagControl != 0 {
		mods = append(mods, "Ctrl")
	}
	if flags&flagOption != 0 {
		mods = append(mods, "Alt")
	}
	if flags&flagShift != 0 {
		mods = append(mods, "Shift")
	}
	if len(mods) == 0 {
		if key == "Escape" {
			return key
		}
		return ""
	}
	return strings.Join(mods, "+") + "+" + key
}

// kept holds the C strings the runtime reads, as a string it may read must
// stay where it is as long as the app runs.
var kept [][]byte

// cstrPtr returns s as a C string that stays for the life of the app.
func cstrPtr(s string) *byte {
	buf := append(make([]byte, 0, len(s)+1), s...)
	buf = append(buf, 0)
	kept = append(kept, buf)
	return &kept[len(kept)-1][0]
}
