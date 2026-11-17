package settings

import (
	"runtime"
	"testing"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", t.TempDir())

	got := Load()
	if got.Shortcut != DefaultShortcut {
		t.Errorf("shortcut = %q, want %q", got.Shortcut, DefaultShortcut)
	}
	if got.StopShortcut != DefaultStopShortcut {
		t.Errorf("stop shortcut = %q, want %q", got.StopShortcut, DefaultStopShortcut)
	}
	if got.LaunchAtLogin {
		t.Error("a fresh install opens at login")
	}

	want := Settings{Shortcut: "CmdOrCtrl+Alt+K", StopShortcut: "CmdOrCtrl+Alt+J", LaunchAtLogin: true}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	if got := Load(); got != want {
		t.Errorf("load = %+v, want %+v", got, want)
	}
}

// TestShortcutKeysReadsAShortcut covers the key of the settings, which macOS
// writes with its symbols and the other systems with words.
func TestShortcutKeysReadsAShortcut(t *testing.T) {
	want := map[string]string{
		"CmdOrCtrl+Shift+S": "Ctrl+Shift+S",
		"CmdOrCtrl+Alt+K":   "Ctrl+Alt+K",
		"Ctrl+Shift+F5":     "Ctrl+Shift+F5",
	}
	if runtime.GOOS == "darwin" {
		want = map[string]string{
			"CmdOrCtrl+Shift+S": "⌘⇧S",
			"CmdOrCtrl+Alt+K":   "⌘⌥K",
			"Ctrl+Shift+F5":     "⌃⇧F5",
		}
	}
	for in, expected := range want {
		if got := shortcutKeys(in); got != expected {
			t.Errorf("shortcutKeys(%q) = %q, want %q", in, got, expected)
		}
	}
}

// TestPickTakesTheCombination covers what the field does with the keys the
// monitor reports.
func TestPickTakesTheCombination(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", t.TempDir())
	value = Settings{Shortcut: DefaultShortcut, StopShortcut: DefaultStopShortcut}
	picking = "capture"
	onApply = nil

	if pick("S", false) {
		t.Error("a key without a modifier was taken")
	}
	if !pick("CmdOrCtrl+Shift+K", false) {
		t.Fatal("a combination was not taken")
	}
	if picking != "" {
		t.Error("the field went on taking keys after a combination")
	}
	if want := "CmdOrCtrl+Shift+K"; value.Shortcut != want {
		t.Errorf("shortcut = %q, want %q", value.Shortcut, want)
	}
	if got := Load().Shortcut; got != "CmdOrCtrl+Shift+K" {
		t.Errorf("on disk = %q, want the new one", got)
	}
}

// TestPickKeepsTheShortcutOnEscape covers the way out of the field.
func TestPickKeepsTheShortcutOnEscape(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", t.TempDir())
	value = Settings{Shortcut: DefaultShortcut, StopShortcut: DefaultStopShortcut}
	picking = "capture"
	onApply = nil

	if !pick("Escape", true) {
		t.Fatal("Escape was not taken")
	}
	if picking != "" {
		t.Error("the field went on taking keys after Escape")
	}
	if value.Shortcut != DefaultShortcut {
		t.Errorf("shortcut = %q, want the one there was", value.Shortcut)
	}
}

// TestPickSetsTheStopShortcut covers the second field, which ends a film.
func TestPickSetsTheStopShortcut(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", t.TempDir())
	value = Settings{Shortcut: DefaultShortcut, StopShortcut: DefaultStopShortcut}
	picking = "stop"
	onApply = nil

	// The shortcut of a capture is the one thing the stop key cannot be.
	if pick(DefaultShortcut, false) {
		t.Error("the stop shortcut took the one of the capture")
	}
	if !pick("CmdOrCtrl+Shift+K", false) {
		t.Fatal("a combination was not taken")
	}
	if want := "CmdOrCtrl+Shift+K"; value.StopShortcut != want {
		t.Errorf("stop shortcut = %q, want %q", value.StopShortcut, want)
	}
	if value.Shortcut != DefaultShortcut {
		t.Errorf("capture shortcut = %q, want it as it was", value.Shortcut)
	}
}
