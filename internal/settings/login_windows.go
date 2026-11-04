//go:build windows

package settings

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

// runKey is where Windows keeps the programs it opens at login.
const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// runValue names the entry of the app there.
const runValue = "Kacha"

// SetLaunchAtLogin opens the app when the user logs in, as an entry of the
// Run key, or takes that entry away.
func SetLaunchAtLogin(on bool) error {
	if !on {
		key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
		if err != nil {
			return nil // no key, nothing to take away
		}
		defer key.Close()
		if err := key.DeleteValue(runValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("login item: %w", err)
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("login item: %w", err)
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("login item: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue(runValue, `"`+exe+`"`); err != nil {
		return fmt.Errorf("login item: %w", err)
	}
	return nil
}

// LaunchAtLogin reports whether the app opens at login now.
func LaunchAtLogin() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetStringValue(runValue)
	return err == nil && value != ""
}
