package shot

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteNamesCapturesByTime(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KACHA_DIR", dir)
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))

	first, err := Write(img, "")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(first) != dir || !strings.HasSuffix(first, ".png") {
		t.Errorf("path = %q, want a PNG in %q", first, dir)
	}
	second, err := Write(img, "")
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Errorf("the second capture overwrote %q", first)
	}
	if _, err := os.Stat(second); err != nil {
		t.Errorf("the second capture is missing: %v", err)
	}
}

func TestWriteIntoADirectory(t *testing.T) {
	dir := t.TempDir()
	path, err := Write(image.NewRGBA(image.Rect(0, 0, 2, 2)), dir+"/nested/")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != filepath.Join(dir, "nested") {
		t.Errorf("path = %q, want a file in %q", path, filepath.Join(dir, "nested"))
	}
}

func TestWriteKeepsTheNameGiven(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shot")
	got, err := Write(image.NewRGBA(image.Rect(0, 0, 2, 2)), path)
	if err != nil {
		t.Fatal(err)
	}
	if want := path + ".png"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestResolveExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // what Windows reads as the home
	got, err := resolve("~/shot.png")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "shot.png"); got != want {
		t.Errorf("resolve = %q, want %q", got, want)
	}
}
