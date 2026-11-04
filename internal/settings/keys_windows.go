//go:build windows

package settings

import (
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/egoist/mygo"
)

// The keys of a shortcut, as Windows numbers them.
const (
	vkEscape  = 0x1B
	vkSpace   = 0x20
	vkPrior   = 0x21
	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12

	whKeyboardLL = 13
	wmKeyDown    = 0x0100
	wmSysKeyDown = 0x0104
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procGetKeyState         = user32.NewProc("GetKeyState")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	procRtlMoveMemory       = kernel32.NewProc("RtlMoveMemory")

	hookOnce   sync.Once
	hookProc   uintptr
	hookHandle uintptr
	hookFn     func(combo string, escape bool) bool
)

// kbStruct is KBDLLHOOKSTRUCT.
type kbStruct struct {
	VkCode    uint32
	ScanCode  uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

// WatchKeys calls fn with every key pressed while the app is in front, as
// an accelerator such as "CmdOrCtrl+Shift+S", with escape told for Escape,
// until the returned func stops it. A true from fn swallows the key.
//
// The hook of Windows runs on the thread that pumps the messages of the
// app, which is the main one.
func WatchKeys(fn func(combo string, escape bool) bool) func() {
	hookOnce.Do(func() { hookProc = syscall.NewCallback(lowLevelKeyboard) })
	hookFn = fn
	mygo.RunOnMain(func() {
		if hookHandle != 0 {
			return
		}
		module, _, _ := procGetModuleHandleW.Call(0)
		h, _, _ := procSetWindowsHookExW.Call(whKeyboardLL, hookProc, module, 0)
		hookHandle = h
	})
	return func() {
		mygo.RunOnMain(func() {
			hookFn = nil
			if hookHandle != 0 {
				procUnhookWindowsHookEx.Call(hookHandle)
				hookHandle = 0
			}
		})
	}
}

// lowLevelKeyboard reads a key the hook hands over, and keeps it from the
// app when the combination it makes is taken.
func lowLevelKeyboard(code uintptr, wparam, lparam uintptr) uintptr {
	if int32(code) < 0 || (wparam != wmKeyDown && wparam != wmSysKeyDown) {
		return nextHook(code, wparam, lparam)
	}
	fn := hookFn
	if fn == nil {
		return nextHook(code, wparam, lparam)
	}
	var key kbStruct
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&key)), lparam, unsafe.Sizeof(key))
	combo := accelerator(key.VkCode)
	if combo == "" {
		return nextHook(code, wparam, lparam)
	}
	if fn(combo, key.VkCode == vkEscape) {
		return 1
	}
	return nextHook(code, wparam, lparam)
}

// nextHook passes the key to the hooks after this one, and the app.
func nextHook(code, wparam, lparam uintptr) uintptr {
	r, _, _ := procCallNextHookEx.Call(hookHandle, code, wparam, lparam)
	return r
}

// accelerator writes a virtual key, with the modifiers held, as the
// accelerator of mygo, such as "CmdOrCtrl+Shift+S", or "" for a key no
// shortcut may end with.
func accelerator(vk uint32) string {
	if vk == vkEscape {
		return "Escape"
	}
	key := keyName(vk)
	if key == "" {
		return ""
	}
	var mods []string
	if down(vkControl) {
		mods = append(mods, "CmdOrCtrl")
	}
	if down(vkMenu) {
		mods = append(mods, "Alt")
	}
	if down(vkShift) {
		mods = append(mods, "Shift")
	}
	if len(mods) == 0 {
		return ""
	}
	return strings.Join(mods, "+") + "+" + key
}

// keyName names the virtual key vk, or "" for one no shortcut may end with.
func keyName(vk uint32) string {
	switch {
	case vk >= 'A' && vk <= 'Z':
		return string(rune(vk))
	case vk >= '0' && vk <= '9':
		return string(rune(vk))
	case vk >= 0x70 && vk <= 0x7B: // F1 to F12
		return "F" + strconv.Itoa(int(vk-0x6F))
	case vk == vkSpace:
		return "Space"
	}
	return ""
}

// down reports whether the key vk is held now.
func down(vk uintptr) bool {
	state, _, _ := procGetKeyState.Call(vk)
	return int16(state) < 0
}
