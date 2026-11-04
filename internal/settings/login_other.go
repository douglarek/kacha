//go:build !darwin && !windows

package settings

// SetLaunchAtLogin does nothing: the app opens at login on macOS and
// Windows alone, where it has a desktop to open on.
func SetLaunchAtLogin(bool) error { return nil }

// LaunchAtLogin reports that the app opens at login nowhere but macOS and
// Windows.
func LaunchAtLogin() bool { return false }
