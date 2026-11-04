// Package shot writes captures to the disk and to the clipboard.
package shot

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// layout names a capture after the moment it was taken.
const layout = "kacha-2006-01-02-15-04-05.png"

// DefaultDir returns $KACHA_DIR, or the Pictures folder of the user.
func DefaultDir() string {
	if dir := os.Getenv("KACHA_DIR"); dir != "" {
		return expand(dir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "Pictures", "Screenshots")
}

// Write saves img at out, which names a file, a directory, or nothing for
// DefaultDir, and returns the path it wrote.
func Write(img image.Image, out string) (string, error) {
	path, err := resolve(out)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("kacha: create %s: %w", filepath.Dir(path), err)
	}
	path = unique(path)
	file, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("kacha: create %s: %w", path, err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		os.Remove(path)
		return "", fmt.Errorf("kacha: write %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("kacha: write %s: %w", path, err)
	}
	return path, nil
}

// Copy puts img on the clipboard as PNG.
func Copy(img image.Image) error {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return fmt.Errorf("kacha: encode for the clipboard: %w", err)
	}
	return copyPNG(buf.Bytes())
}

// resolve turns out into the file the capture goes to.
func resolve(out string) (string, error) {
	name := time.Now().Format(layout)
	if out == "" {
		return filepath.Join(DefaultDir(), name), nil
	}
	path := expand(out)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return filepath.Join(path, name), nil
	}
	if strings.HasSuffix(out, "/") {
		return filepath.Join(path, name), nil
	}
	if filepath.Ext(path) == "" {
		path += ".png"
	}
	return path, nil
}

// unique appends a number to path until no file has it.
func unique(path string) string {
	if _, err := os.Stat(path); err != nil {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(candidate); err != nil {
			return candidate
		}
	}
}

// expand replaces a leading ~ with the home directory.
func expand(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return path
}
