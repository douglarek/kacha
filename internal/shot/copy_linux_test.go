//go:build linux && (amd64 || arm64)

package shot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCopyHandsThePNGToWLCopy checks that a Wayland session copies through
// wl-copy, whose process keeps serving the selection after kacha exits.
func TestCopyHandsThePNGToWLCopy(t *testing.T) {
	dir := t.TempDir()
	shared, argv := filepath.Join(dir, "shared"), filepath.Join(dir, "argv")
	script := "#!/bin/sh\nprintf '%s' \"$*\" > " + argv + "\ncat > " + shared + "\n"
	if err := os.WriteFile(filepath.Join(dir, "wl-copy"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	wrote := stubClipboard(t)

	data := []byte("a capture")
	if err := copyPNG(data); err != nil {
		t.Fatal(err)
	}
	if *wrote {
		t.Error("copyPNG copied through GTK as well")
	}
	got, err := os.ReadFile(shared)
	if err != nil {
		t.Fatalf("wl-copy wrote nothing: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("wl-copy read %q, want %q", got, data)
	}
	args, err := os.ReadFile(argv)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(args)); got != "-t image/png" {
		t.Errorf("wl-copy ran with %q, want -t image/png", got)
	}
}

// TestCopyDoesNotWaitForTheProcessWLCopyLeaves checks that a wl-copy which
// forks, as the real one does to serve the selection, does not hold the
// caller: waiting on that process left the overlay frozen until the
// compositor dropped the client.
func TestCopyDoesNotWaitForTheProcessWLCopyLeaves(t *testing.T) {
	dir := t.TempDir()
	shared := filepath.Join(dir, "shared")
	script := "#!/bin/sh\ncat > " + shared + "\n( sleep 3 ) &\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "wl-copy"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	stubClipboard(t)

	start := time.Now()
	if err := copyPNG([]byte("a capture")); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > time.Second {
		t.Errorf("copyPNG took %v, want the fork alone", took)
	}
}

// TestCopyFallsBackToGTKWithoutWLCopy checks that a Wayland session without
// wl-clipboard still copies.
func TestCopyFallsBackToGTKWithoutWLCopy(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	wrote := stubClipboard(t)

	if err := copyPNG([]byte("a capture")); err != nil {
		t.Fatal(err)
	}
	if !*wrote {
		t.Error("copyPNG did not fall back to GTK")
	}
}

// TestCopyUsesGTKWithoutWayland checks that an X11 session copies through GTK
// alone, as wl-copy has no server there.
func TestCopyUsesGTKWithoutWayland(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "wl-copy"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("WAYLAND_DISPLAY", "")
	wrote := stubClipboard(t)

	if err := copyPNG([]byte("a capture")); err != nil {
		t.Fatal(err)
	}
	if !*wrote {
		t.Error("copyPNG did not copy through GTK")
	}
}

// stubClipboard replaces the GTK write and reports whether it was called.
func stubClipboard(t *testing.T) *bool {
	t.Helper()
	original := writeClipboard
	wrote := false
	writeClipboard = func([]byte) error {
		wrote = true
		return nil
	}
	t.Cleanup(func() { writeClipboard = original })
	return &wrote
}
