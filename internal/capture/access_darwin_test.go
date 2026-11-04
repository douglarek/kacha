//go:build darwin

package capture

import (
	"errors"
	"strings"
	"testing"
)

func TestAccessErrorNamesTheCopy(t *testing.T) {
	t.Setenv("__CFBundleIdentifier", "")
	err := accessErrorFor("/Applications/kacha.app/Contents/MacOS/kacha")
	if !errors.Is(err, ErrDenied) {
		t.Errorf("err = %v, want ErrDenied", err)
	}
	msg := err.Error()
	for _, want := range []string{
		"/Applications/kacha.app/Contents/MacOS/kacha",
		"Screen & System Audio Recording",
		"rebuilding an unsigned app",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
}

func TestAccessErrorBlamesTheAppThatStartedIt(t *testing.T) {
	t.Setenv("__CFBundleIdentifier", "com.example.Terminal")
	msg := accessErrorFor("/Applications/kacha.app/Contents/MacOS/kacha").Error()
	for _, want := range []string{"com.example.Terminal", "open -a Kacha"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
}

func TestAccessErrorExplainsTranslocation(t *testing.T) {
	const exe = "/private/var/folders/xy/AppTranslocation/9F0E/d/kacha.app/Contents/MacOS/kacha"
	msg := accessErrorFor(exe).Error()
	if !strings.Contains(msg, "translocated") || !strings.Contains(msg, "/Applications") {
		t.Errorf("a translocated copy is not explained:\n%s", msg)
	}
}
