//go:build linux && (amd64 || arm64)

package capture

import (
	"fmt"
	"os"
)

// Detect returns the desktop portal, the one capture path every Wayland
// desktop implements.
func Detect() (Backend, error) {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return nil, fmt.Errorf("%w: this session runs X11, and kacha captures Wayland sessions through the desktop portal", ErrUnsupported)
	}
	portal := portalBackend{}
	if !portal.available() {
		return nil, fmt.Errorf("%w: WAYLAND_DISPLAY is set but no screenshot portal answers; install xdg-desktop-portal, and its backend for this compositor (xdg-desktop-portal-wlr on wlroots)", ErrUnsupported)
	}
	return portal, nil
}
