// Package settings keeps the choices kacha remembers between runs: the
// shortcut that starts a capture, and whether the app opens at login.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DefaultShortcut starts a capture until the user picks another.
const DefaultShortcut = "CmdOrCtrl+Shift+S"

// Settings are the choices of the settings window.
type Settings struct {
	// Shortcut starts a capture from anywhere, in the syntax of menu
	// accelerators, e.g. "CmdOrCtrl+Shift+S".
	Shortcut string `json:"shortcut"`
	// LaunchAtLogin opens the app when the user logs in.
	LaunchAtLogin bool `json:"launchAtLogin"`
}

// Load returns the settings on the disk, with the defaults filled in.
func Load() Settings {
	s := Settings{Shortcut: DefaultShortcut}
	file, err := os.Open(path())
	if err != nil {
		return s
	}
	defer file.Close()
	_ = json.NewDecoder(file).Decode(&s)
	if s.Shortcut == "" {
		s.Shortcut = DefaultShortcut
	}
	return s
}

// Save writes s, and makes the app open at login or not, as s says.
func Save(s Settings) error {
	dir := filepath.Dir(path())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path(), append(data, '\n'), 0o644); err != nil {
		return err
	}
	if LaunchAtLogin() == s.LaunchAtLogin {
		return nil
	}
	return SetLaunchAtLogin(s.LaunchAtLogin)
}

// path returns the file the settings live in.
func path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "kacha", "config.json")
}
