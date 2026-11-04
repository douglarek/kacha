//go:build !darwin && !windows

package settings

// WatchKeys does nothing outside macOS, where the settings are of mygo.
func WatchKeys(fn func(combo string, escape bool) bool) func() { return func() {} }
