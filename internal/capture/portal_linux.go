//go:build linux && (amd64 || arm64)

package capture

import (
	"context"
	"errors"
	"fmt"
	"image"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	portalBus        = "org.freedesktop.portal.Desktop"
	portalObjectPath = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	portalScreenshot = "org.freedesktop.portal.Screenshot"
	portalRequest    = "org.freedesktop.portal.Request"
	requestPrefix    = "/org/freedesktop/portal/desktop/request/"
)

// portalBackend captures through org.freedesktop.portal.Screenshot, which
// every Wayland desktop implements.
type portalBackend struct{}

// Name identifies the backend.
func (portalBackend) Name() string { return "portal" }

// available reports whether a screenshot portal answers on the session bus.
func (portalBackend) available() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	var owner bool
	err = conn.Object("org.freedesktop.DBus", dbus.ObjectPath("/org/freedesktop/DBus")).
		Call("org.freedesktop.DBus.NameHasOwner", 0, portalBus).Store(&owner)
	return err == nil && owner
}

// Capture takes the screenshot the portal returns, which covers the whole
// desktop.
func (p portalBackend) Capture(ctx context.Context) (image.Image, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("capture: session bus: %w", err)
	}
	names := conn.Names()
	if len(names) == 0 {
		return nil, errors.New("capture: the session bus gives no name to this process")
	}
	token := portalToken()
	request := requestPath(names[0], token)

	responses := make(chan *dbus.Signal, 4)
	conn.Signal(responses)
	defer conn.RemoveSignal(responses)
	match := func(path dbus.ObjectPath) error {
		return conn.AddMatchSignal(
			dbus.WithMatchObjectPath(path),
			dbus.WithMatchInterface(portalRequest),
			dbus.WithMatchMember("Response"),
		)
	}
	if err := match(request); err != nil {
		return nil, fmt.Errorf("capture: watch portal request: %w", err)
	}
	defer conn.RemoveMatchSignal(
		dbus.WithMatchObjectPath(request),
		dbus.WithMatchInterface(portalRequest),
		dbus.WithMatchMember("Response"),
	)

	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
		"interactive":  dbus.MakeVariant(false),
		"modal":        dbus.MakeVariant(false),
		"reason":       dbus.MakeVariant("Take a screenshot"),
	}
	var handle dbus.ObjectPath
	call := conn.Object(portalBus, portalObjectPath).
		CallWithContext(ctx, portalScreenshot+".Screenshot", 0, "", options)
	if err := call.Store(&handle); err != nil {
		var busError dbus.Error
		if errors.As(err, &busError) && strings.HasPrefix(busError.Name, "org.freedesktop.DBus.Error.Unknown") {
			return nil, fmt.Errorf("%w: this portal does not implement Screenshot, so the desktop needs its screenshot backend installed: %w", ErrUnsupported, err)
		}
		return nil, fmt.Errorf("capture: portal screenshot: %w", err)
	}
	if handle != "" && handle != request {
		if err := match(handle); err != nil {
			return nil, fmt.Errorf("capture: watch portal request: %w", err)
		}
		defer conn.RemoveMatchSignal(
			dbus.WithMatchObjectPath(handle),
			dbus.WithMatchInterface(portalRequest),
			dbus.WithMatchMember("Response"),
		)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case signal := <-responses:
			if signal == nil {
				return nil, errors.New("capture: the portal connection closed")
			}
			if signal.Name != portalRequest+".Response" {
				continue
			}
			if signal.Path != request && signal.Path != handle {
				continue
			}
			code, results, err := portalResponse(signal.Body)
			if err != nil {
				return nil, err
			}
			switch code {
			case 0:
			case 1:
				return nil, ErrDenied
			case 2:
				return nil, fmt.Errorf("%w: the portal answered %d", ErrUnsupported, code)
			default:
				return nil, fmt.Errorf("capture: the portal answered %d", code)
			}
			uri, _ := results["uri"].Value().(string)
			return readFileURI(uri)
		}
	}
}

// portalToken names a request, in the characters a handle_token allows.
func portalToken() string {
	return "kacha" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

// requestPath returns the object path a portal answers a request on, which
// names the sender that made it and the token of its handle_token.
func requestPath(sender, token string) dbus.ObjectPath {
	return dbus.ObjectPath(requestPrefix +
		strings.ReplaceAll(strings.TrimPrefix(sender, ":"), ".", "_") +
		"/" + token)
}

// portalResponse reads the code and results of a request's Response signal.
func portalResponse(body []any) (uint32, map[string]dbus.Variant, error) {
	if len(body) != 2 {
		return 0, nil, fmt.Errorf("capture: portal response has %d fields", len(body))
	}
	code, ok := body[0].(uint32)
	if !ok {
		return 0, nil, fmt.Errorf("capture: portal response code is %T", body[0])
	}
	results, _ := body[1].(map[string]dbus.Variant)
	return code, results, nil
}

// readFileURI decodes the image a file:// URI names.
func readFileURI(uri string) (image.Image, error) {
	if uri == "" {
		return nil, errors.New("capture: the portal returned no screenshot")
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("capture: screenshot URI: %w", err)
	}
	file, err := os.Open(u.Path)
	if err != nil {
		return nil, fmt.Errorf("capture: screenshot: %w", err)
	}
	defer file.Close()
	return decode(file)
}
