//go:build !darwin && !windows

package capture

// Windows returns no window: Wayland tells an application nothing about the
// windows of other applications, so the selection stays a rectangle.
func Windows() ([]Window, error) { return nil, nil }
