//go:build linux

package record

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// The portal every Wayland desktop runs, and the interface that shares a
// screen for a film.
const (
	portalBus        = "org.freedesktop.portal.Desktop"
	portalObjectPath = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	portalScreencast = "org.freedesktop.portal.ScreenCast"
	portalRequest    = "org.freedesktop.portal.Request"
	requestPrefix    = "/org/freedesktop/portal/desktop/request/"
)

// ErrDenied reports that the user turned the film down, at the picker of the
// desktop.
var ErrDenied = errors.New("kacha: record: the film was turned down")

// share is what the desktop shares with the recorder: a PipeWire stream, the
// size of its frames, and the connection it is read on.
type share struct {
	node uint32
	size image.Point
	conn *os.File
}

// Close hangs up the share, which ends the stream.
func (s *share) Close() {
	if s == nil || s.conn == nil {
		return
	}
	_ = s.conn.Close()
	s.conn = nil
}

// openShare asks the desktop to share a screen or a window, and returns the
// stream it shares: the node of a PipeWire stream, and the connection the
// portal opens for it, which is what gives the recorder the right to read it.
//
// The desktop shows its own picker, so this lasts as long as the user takes
// to pick.
func openShare(ctx context.Context) (*share, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("record: session bus: %w", err)
	}
	p := &portal{conn: conn}

	// A session holds what the desktop shares, and the picker it showed.
	results, err := p.request(ctx, portalScreencast+".CreateSession",
		map[string]dbus.Variant{
			"handle_token":         dbus.MakeVariant(portalToken()),
			"session_handle_token": dbus.MakeVariant(portalToken()),
		})
	if err != nil {
		return nil, err
	}
	session := sessionHandle(results)
	if session == "" {
		return nil, errors.New("record: the portal opened no session")
	}

	// The sources: a screen, which the film crops to the area the user
	// picked, with the pointer in it where the stream carries the pointer.
	if _, err := p.request(ctx, portalScreencast+".SelectSources", session,
		map[string]dbus.Variant{
			"handle_token": dbus.MakeVariant(portalToken()),
			"types":        dbus.MakeVariant(uint32(1)),
			"multiple":     dbus.MakeVariant(false),
			"cursor_mode":  dbus.MakeVariant(uint32(2)),
		}); err != nil {
		return nil, err
	}

	// Start asks the user to pick, and tells which stream they picked.
	results, err = p.request(ctx, portalScreencast+".Start", session, "",
		map[string]dbus.Variant{"handle_token": dbus.MakeVariant(portalToken())})
	if err != nil {
		return nil, err
	}
	node, size, err := streamInfo(results)
	if err != nil {
		return nil, err
	}

	// The connection to those streams, which the desktop hands out as a file
	// descriptor: a stream of the portal is invisible to any other client.
	var fd dbus.UnixFD
	if err := conn.Object(portalBus, portalObjectPath).
		CallWithContext(ctx, portalScreencast+".OpenPipeWireRemote", 0, session,
			map[string]dbus.Variant{}).Store(&fd); err != nil {
		return nil, fmt.Errorf("record: portal pipewire: %w", err)
	}
	if fd < 0 {
		return nil, errors.New("record: the portal opened no pipewire connection")
	}
	return &share{node: node, size: size, conn: os.NewFile(uintptr(fd), "pipewire")}, nil
}

// sessionHandle returns the object path of the session the portal opened,
// which its answer names as a string.
func sessionHandle(results map[string]dbus.Variant) dbus.ObjectPath {
	switch said := results["session_handle"].Value().(type) {
	case string:
		return dbus.ObjectPath(said)
	case dbus.ObjectPath:
		return said
	}
	return ""
}

// portalAvailable reports whether the desktop shares its screen through the
// portal of the session bus.
func portalAvailable() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	var owner bool
	err = conn.Object("org.freedesktop.DBus", dbus.ObjectPath("/org/freedesktop/DBus")).
		Call("org.freedesktop.DBus.NameHasOwner", 0, portalBus).Store(&owner)
	return err == nil && owner
}

// streamInfo returns the node of the first stream the portal started, and
// the size of its frames, which the desktop knows and the film needs to crop
// the area out of it.
func streamInfo(results map[string]dbus.Variant) (uint32, image.Point, error) {
	said, ok := results["streams"]
	if !ok {
		return 0, image.Point{}, errors.New("record: the portal started no stream")
	}
	streams := lists(said.Value())
	if len(streams) == 0 {
		return 0, image.Point{}, errors.New("record: the portal started no stream")
	}
	// The result is a(ua{sv}): a node, and what the desktop says of it.
	first := lists(streams[0])
	if len(first) == 0 {
		return 0, image.Point{}, errors.New("record: the portal started no stream")
	}
	node, ok := number(first[0])
	if !ok || node <= 0 {
		return 0, image.Point{}, fmt.Errorf("record: the portal named a stream of %T", first[0])
	}
	var size image.Point
	if len(first) > 1 {
		if props, ok := first[1].(map[string]dbus.Variant); ok {
			size = streamSize(props)
		}
	}
	return uint32(node), size, nil
}

// lists returns what the marshaller made of a list of the portal, which is a
// slice of any type of its elements.
func lists(value any) []any {
	switch list := value.(type) {
	case []any:
		return list
	case [][]any:
		out := make([]any, len(list))
		for i, element := range list {
			out[i] = element
		}
		return out
	}
	return nil
}

// number returns the value of an integer of a list of the portal.
func number(value any) (int64, bool) {
	switch n := value.(type) {
	case uint32:
		return int64(n), true
	case int32:
		return int64(n), true
	case uint64:
		return int64(n), n <= math.MaxInt64
	case int64:
		return n, true
	}
	return 0, false
}

// pair returns the two integers of a list of the portal, which it writes as
// a struct of two.
func pair(value any) (int, int, bool) {
	switch p := value.(type) {
	case []any:
		if len(p) != 2 {
			return 0, 0, false
		}
		first, ok := number(p[0])
		if !ok {
			return 0, 0, false
		}
		second, ok := number(p[1])
		if !ok {
			return 0, 0, false
		}
		return int(first), int(second), true
	case []int32:
		if len(p) != 2 {
			return 0, 0, false
		}
		return int(p[0]), int(p[1]), true
	case []int64:
		if len(p) != 2 {
			return 0, 0, false
		}
		return int(p[0]), int(p[1]), true
	}
	return 0, 0, false
}

// streamSize returns the size of the frames of a stream, from what the
// desktop says of it, or nothing where it says nothing the film can use.
func streamSize(props map[string]dbus.Variant) image.Point {
	said, ok := props["size"]
	if !ok {
		return image.Point{}
	}
	width, height, ok := pair(said.Value())
	if !ok || width <= 0 || height <= 0 {
		return image.Point{}
	}
	return image.Pt(width, height)
}

// portal calls the methods of the ScreenCast interface, which answer a
// request the desktop answers later, as the user picks.
type portal struct{ conn *dbus.Conn }

// request calls a method whose answer is a response to a request, and returns
// what the answer holds.
func (p *portal) request(ctx context.Context, method string, args ...any) (map[string]dbus.Variant, error) {
	names := p.conn.Names()
	if len(names) == 0 {
		return nil, errors.New("record: the session bus gives no name to this process")
	}
	token := portalToken()
	request := requestPath(names[0], token)

	responses := make(chan *dbus.Signal, 4)
	p.conn.Signal(responses)
	defer p.conn.RemoveSignal(responses)
	match := func(path dbus.ObjectPath) error {
		return p.conn.AddMatchSignal(
			dbus.WithMatchObjectPath(path),
			dbus.WithMatchInterface(portalRequest),
			dbus.WithMatchMember("Response"),
		)
	}
	if err := match(request); err != nil {
		return nil, fmt.Errorf("record: watch portal request: %w", err)
	}
	defer p.conn.RemoveMatchSignal(
		dbus.WithMatchObjectPath(request),
		dbus.WithMatchInterface(portalRequest),
		dbus.WithMatchMember("Response"),
	)

	var handle dbus.ObjectPath
	call := p.conn.Object(portalBus, portalObjectPath).CallWithContext(ctx, method, 0, args...)
	if err := call.Store(&handle); err != nil {
		var busError dbus.Error
		if errors.As(err, &busError) && strings.HasPrefix(busError.Name, "org.freedesktop.DBus.Error.Unknown") {
			return nil, fmt.Errorf("%w: this desktop does not share its screen through the portal: %w", ErrUnsupported, err)
		}
		return nil, fmt.Errorf("record: portal %s: %w", method, err)
	}
	if handle != "" && handle != request {
		if err := match(handle); err != nil {
			return nil, fmt.Errorf("record: watch portal request: %w", err)
		}
		defer p.conn.RemoveMatchSignal(
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
				return nil, errors.New("record: the portal connection closed")
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
				return results, nil
			case 1:
				return nil, ErrDenied
			default:
				return nil, fmt.Errorf("record: the portal answered %d", code)
			}
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
		return 0, nil, fmt.Errorf("record: portal response has %d fields", len(body))
	}
	code, ok := body[0].(uint32)
	if !ok {
		return 0, nil, fmt.Errorf("record: portal response code is %T", body[0])
	}
	results, _ := body[1].(map[string]dbus.Variant)
	return code, results, nil
}
