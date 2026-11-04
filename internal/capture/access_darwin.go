//go:build darwin

package capture

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/purego"
)

// Allowed reports whether this copy of kacha may record the screen. It
// answers without asking the user, unlike RequestAccess.
func Allowed() bool {
	preflightOnce.Do(func() {
		lib, err := purego.Dlopen(
			"/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics",
			purego.RTLD_NOW|purego.RTLD_GLOBAL,
		)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&preflight, lib, "CGPreflightScreenCaptureAccess")
	})
	return preflight == nil || preflight()
}

var (
	preflightOnce sync.Once
	preflight     func() bool

	requestOnce sync.Once
	request     func() bool
)

// ours returns the bundle identifier of the app that holds exe, "" when exe
// is not in an app bundle, or none reads.
func ours(exe string) string {
	dir := filepath.Dir(exe)
	for range 3 {
		dir = filepath.Dir(dir)
		if strings.HasSuffix(dir, ".app") {
			return plistValue(filepath.Join(dir, "Contents", "Info.plist"), "CFBundleIdentifier")
		}
	}
	return ""
}

// plistValue returns the string a key of a property list holds.
func plistValue(path, key string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	dec := xml.NewDecoder(file)
	last := ""
	for {
		token, err := dec.Token()
		if err != nil {
			return ""
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "key":
			var k string
			if dec.DecodeElement(&k, &start) == nil {
				last = k
			}
		case "string":
			if last != key {
				continue
			}
			var v string
			if dec.DecodeElement(&v, &start) == nil {
				return v
			}
		}
	}
}

// RequestAccess asks the system to let this copy record the screen, which
// shows its permission dialog, and reports the answer.
func RequestAccess() (bool, error) {
	requestOnce.Do(func() {
		lib, err := purego.Dlopen(
			"/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics",
			purego.RTLD_NOW|purego.RTLD_GLOBAL,
		)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&request, lib, "CGRequestScreenCaptureAccess")
	})
	if request == nil {
		return true, nil
	}
	return request(), nil
}

// Identity is what a capture needs to know about the running copy, for
// telling one copy from another.
type Identity struct {
	// Path is the executable that runs.
	Path string
	// Bundle is the identifier of the app that holds it, if any.
	Bundle string
	// Host is the identifier of the app that started it, if another one did.
	Host string
	// Allowed reports whether the copy may record the screen.
	Allowed bool
}

// Identify describes the running copy.
func Identify() Identity {
	exe, err := os.Executable()
	if err != nil {
		exe = "kacha"
	}
	id := Identity{Path: exe, Bundle: ours(exe), Allowed: Allowed()}
	if host, _ := os.LookupEnv("__CFBundleIdentifier"); host != id.Bundle {
		id.Host = host
	}
	return id
}

// WaitAllowed waits until this copy may record the screen, as the user
// answers the system's prompt, or until ctx is done.
func WaitAllowed(ctx context.Context) bool {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if Allowed() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

// accessError explains what keeps this copy from recording the screen.
func accessError() error {
	exe, err := os.Executable()
	if err != nil {
		exe = "kacha"
	}
	return accessErrorFor(exe)
}

// accessErrorFor explains what keeps the copy at exe from recording the
// screen.
func accessErrorFor(exe string) error {
	if strings.Contains(exe, "/AppTranslocation/") {
		return fmt.Errorf("%w: macOS runs this copy from %s, a translocated location, where a permission never sticks: move kacha.app to /Applications, open it from there, and allow it once", ErrDenied, exe)
	}
	if host := os.Getenv("__CFBundleIdentifier"); host != "" && host != ours(exe) {
		return fmt.Errorf("%w: %s started kacha, so macOS asks that app for the Screen Recording permission, not kacha: allow %[2]s in System Settings › Privacy & Security › Screen & System Audio Recording, or open kacha on its own with `open -a Kacha`", ErrDenied, host)
	}
	return fmt.Errorf("%w: %s may not record the screen. Allow it in System Settings › Privacy & Security › Screen & System Audio Recording, quit kacha, then open it again; run `kacha permission` to make the system ask. The system grants the permission to one build of one copy: rebuilding an unsigned app, running it with `go run`, or opening a second copy elsewhere each ask again", ErrDenied, exe)
}
