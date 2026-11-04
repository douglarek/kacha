//go:build linux && (amd64 || arm64)

package capture

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestRequestPath(t *testing.T) {
	got := requestPath(":1.42", "kachaabc")
	want := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/1_42/kachaabc")
	if got != want {
		t.Errorf("requestPath = %q, want %q", got, want)
	}
}

func TestPortalToken(t *testing.T) {
	token := portalToken()
	if len(token) < len("kacha")+1 {
		t.Fatalf("token %q carries no number", token)
	}
	for _, r := range token {
		if r < '0' || r > '9' && (r < 'a' || r > 'z') {
			t.Errorf("token %q has %q, which a handle_token does not allow", token, r)
		}
	}
}

func TestPortalResponse(t *testing.T) {
	body := []any{uint32(0), map[string]dbus.Variant{
		"uri": dbus.MakeVariant("file:///tmp/kacha.png"),
	}}
	code, results, err := portalResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
	if uri, _ := results["uri"].Value().(string); uri != "file:///tmp/kacha.png" {
		t.Errorf("uri = %q", uri)
	}
	if _, _, err := portalResponse([]any{uint32(1)}); err == nil {
		t.Error("a response of one field was accepted")
	}
	if _, _, err := portalResponse([]any{"zero", nil}); err == nil {
		t.Error("a response whose code is not a number was accepted")
	}
}

func TestReadFileURI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shot.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 7, 5))); err != nil {
		t.Fatal(err)
	}
	file.Close()

	img, err := readFileURI("file://" + path)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds() != image.Rect(0, 0, 7, 5) {
		t.Errorf("bounds = %v, want 7x5", img.Bounds())
	}
	if _, err := readFileURI(""); err == nil {
		t.Error("an empty URI was accepted")
	}
	if _, err := readFileURI("file:///nonexistent/kacha.png"); err == nil {
		t.Error("a missing file was accepted")
	}
}
