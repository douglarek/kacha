// Command kacha captures the screen of a Wayland session, of macOS, and of
// Windows.
package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/douglarek/kacha/internal/capture"
	"github.com/douglarek/kacha/internal/editor"
	"github.com/douglarek/kacha/internal/overlay"
	"github.com/douglarek/kacha/internal/region"
	"github.com/douglarek/kacha/internal/settings"
	"github.com/douglarek/kacha/internal/shot"
	"github.com/douglarek/kacha/locales"
	"github.com/egoist/mygo"
)

// config is mygo.json, which the binary carries: the version it names is the
// one the app reports, and the one a package is named after.
//
//go:embed mygo.json
var config []byte

// version is the release a build stamps into the binary with -ldflags -X, as
// MYGO_LDFLAGS does. A build that stamps none reports the version mygo.json
// names, so that there is one place to change it.
var version string

// buildVersion returns the version of this build.
func buildVersion() string {
	if version != "" {
		return version
	}
	var app struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(config, &app); err != nil || app.Version == "" {
		return "unknown"
	}
	return app.Version
}

const usage = `kacha captures the screen and puts it on the clipboard.

Usage:
  kacha                    opened on its own, stay in the tray (Windows)
  kacha [region]           capture the screen, pick a part, annotate it
  kacha full               capture the whole display under the pointer
  kacha area X,Y,WxH       capture a rectangle of that display, in DIPs
  kacha settings           open the settings: the shortcut and opening at login
  kacha permission         ask macOS for permission to record the screen
  kacha version            print the version

Flags:
  -o, --out PATH   save at PATH, a file or a directory, and turn saving on
                   (default $KACHA_DIR, else ~/Pictures/Screenshots)
  --stdout         write the PNG to stdout instead of saving and copying
  --copy=BOOL      copy the capture to the clipboard (default true)
  --save=BOOL      write the capture to the disk (default false)
  --delay D        wait D before capturing, as 1.5s
  --area SPEC      the rectangle of the area command

Env:
  KACHA_DIR        where captures go unless --out says otherwise
  KACHA_SOURCE     capture from this PNG instead of the screen
  KACHA_THEME      light or dark, for the toolbar (default: the desktop's)
`

func main() {
	err := run(os.Args[1:])
	switch {
	case err == nil:
	case errors.Is(err, overlay.ErrCanceled):
		os.Exit(130)
	default:
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// options are the command line of one run.
type options struct {
	cmd     string
	area    string
	out     string
	stdout  bool
	copy    bool
	save    bool
	saveSet bool
	delay   time.Duration
}

func run(args []string) error {
	switch strings.ToLower(os.Getenv("KACHA_THEME")) {
	case "light":
		editor.ForcedTheme = 1
	case "dark":
		editor.ForcedTheme = -1
	}
	cmd := "region"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	// A command is a word, but spelling one as a flag is what a shell
	// completion and a habit make of it.
	if name, ok := strings.CutPrefix(cmd, "--"); ok && name != "" && !strings.Contains(name, "=") {
		cmd = name
	}
	switch cmd {
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	case "version":
		fmt.Println("kacha", buildVersion())
		return nil
	case "settings":
		if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
			return errors.New("kacha: settings exist on macOS and Windows alone, where the app opens at login and holds the shortcut")
		}
		return gui(options{cmd: "settings", copy: true})
	case "permission":
		return permission()
	}
	o := options{cmd: cmd, copy: true}
	if cmd == "area" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		o.area, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("kacha", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	fs.StringVar(&o.out, "o", "", "where the capture goes")
	fs.StringVar(&o.out, "out", "", "where the capture goes")
	fs.StringVar(&o.area, "area", o.area, "the rectangle of the area command")
	fs.BoolVar(&o.stdout, "stdout", false, "write the PNG to stdout")
	fs.BoolVar(&o.copy, "copy", true, "copy the capture to the clipboard")
	fs.BoolVar(&o.save, "save", false, "write the capture to the disk")
	fs.DurationVar(&o.delay, "delay", 0, "wait before capturing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	fs.Visit(func(f *flag.Flag) {
		o.saveSet = o.saveSet || f.Name == "save"
	})
	switch o.cmd {
	case "region", "full":
	case "area":
		if o.area == "" {
			return errors.New("kacha: area takes X,Y,WxH")
		}
	case "settings":
	default:
		return fmt.Errorf("kacha: unknown command %q, see kacha help", o.cmd)
	}
	if o.stdout {
		return pipe(o)
	}
	return gui(o)
}

// docked reports whether this copy runs as the app of a Dock, rather than
// from a terminal, where it takes one capture and leaves.
// openedAlone reports whether a desktop opened the app, with no arguments
// at all, which is how Windows opens what a click on it starts.
func openedAlone() bool { return runtime.GOOS == "windows" && len(os.Args) == 1 }

func docked() bool {
	return os.Getenv("__CFBundleIdentifier") == "vibe.kacha"
}

//go:embed resources/icon.png
var appIcon []byte

// trayIcon returns what the tray of this platform shows: the template glyph
// of the menu bar of macOS, which tints it, or the icon of the app, which
// Windows draws as it is.
func trayIcon() []byte {
	if runtime.GOOS == "darwin" {
		return trayGlyph()
	}
	return appIcon
}

// trayIsTemplate reports whether the tray tints the image, as the menu bar
// of macOS does.
func trayIsTemplate() bool { return runtime.GOOS == "darwin" }

// trayGlyph draws the menu bar item: the corner brackets and dot of the app
// icon, as heavy as the icons macOS draws there. mygo shows the image at the
// 18 points of a menu bar, which 36 pixels fill on a Retina display.
func trayGlyph() []byte {
	const size = 36
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	black := color.NRGBA{A: 255}
	// The ink keeps a margin inside the image, as the icons beside it do.
	const inset, arm, thick, radius = 5, 9, 4, 3
	for _, corner := range [][2]int{{inset, inset}, {size - 1 - inset, inset}, {inset, size - 1 - inset}, {size - 1 - inset, size - 1 - inset}} {
		dx, dy := 1, 1
		if corner[0] > size/2 {
			dx = -1
		}
		if corner[1] > size/2 {
			dy = -1
		}
		for i := range arm {
			for t := range thick {
				img.Set(corner[0]+dx*i, corner[1]+dy*t, black)
				img.Set(corner[0]+dx*t, corner[1]+dy*i, black)
			}
		}
	}
	for y := range size {
		for x := range size {
			dx, dy := x-size/2, y-size/2
			if dx*dx+dy*dy <= radius*radius {
				img.Set(x, y, black)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// logf appends a line to the log of the app, which a windowless launch has
// no terminal to print it to.
func logf(format string, args ...any) {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(filepath.Join(dir, "kacha"), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(filepath.Join(dir, "kacha", "kacha.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	fmt.Fprintf(file, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

// permission asks the system for what a capture needs, which on macOS is
// the screen recording permission of this copy.
func permission() error {
	id := capture.Identify()
	fmt.Printf("binary: %s\n", id.Path)
	if id.Bundle != "" {
		fmt.Printf("bundle: %s\n", id.Bundle)
	}
	if id.Host != "" {
		fmt.Printf("started by: %s\n", id.Host)
	}
	if capture.Allowed() {
		fmt.Println("kacha may record the screen; quit and open it again where a capture still fails")
		return nil
	}
	ok, err := capture.RequestAccess()
	if err != nil {
		return err
	}
	if ok {
		fmt.Println("kacha may record the screen now; quit and open it again")
		return nil
	}
	fmt.Println("kacha still may not record the screen: allow it in System Settings › Privacy & Security › Screen & System Audio Recording, then quit and open kacha again")
	return nil
}

// pipe captures without a window and writes the PNG to stdout.
func pipe(o options) error {
	if o.cmd != "full" {
		return errors.New("kacha: --stdout writes the whole screen, so it takes the full command")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	img, err := grab(ctx, o)
	if err != nil {
		return err
	}
	return png.Encode(os.Stdout, img)
}

// gui runs the capture inside an application, which the overlay, the
// displays and the clipboard all need.
func gui(o options) error {
	overlay.Logf = logf
	settings.Logf = logf
	logf("start docked=%v alone=%v args=%v", docked(), openedAlone(), os.Args[1:])
	// A tool of the menu bar takes no place in the Dock, and, what matters,
	// comes forward for no capture it has not taken yet: an app that is
	// front when the screen is captured hides the panels of the one it
	// replaced, and they would be missing from the capture.
	if runtime.GOOS == "darwin" {
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	}
	// Opened on its own, with no arguments at all, the app stays in the
	// tray of Windows, as the app of macOS does. A shell that wants a
	// capture names one: kacha region, kacha full, kacha area.
	if docked() || o.cmd == "settings" || openedAlone() {
		return resident(o)
	}
	errs := make(chan error, 1)
	mygo.App.WhenReady(func() {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logf("panic: %v\n%s", r, debug.Stack())
					errs <- fmt.Errorf("kacha: %v", r)
					mygo.App.Quit()
				}
			}()
			err := shoot(o)
			if err != nil {
				logf("error: %v", err)
			}
			errs <- err
			mygo.App.Quit()
		}()
	})
	mygo.App.OnWindowAllClosed(func() {})
	if err := mygo.App.Run(); err != nil {
		return err
	}
	return <-errs
}

// resident runs the app of a Dock: it captures when asked, from its menu bar
// item or its shortcut, and stays for the next one.
func resident(o options) error {
	// One app, one tray: a launch that finds another one hands its command
	// line over and goes away.
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	var busy atomic.Bool
	capture := func() {
		if !busy.CompareAndSwap(false, true) {
			return
		}
		go func() {
			defer busy.Store(false)
			defer func() {
				if r := recover(); r != nil {
					logf("panic: %v\n%s", r, debug.Stack())
					mygo.Dialog.Error(locales.CaptureFailed(), fmt.Sprint(r))
				}
			}()
			logf("capture start")
			if err := shoot(o); err != nil && !errors.Is(err, overlay.ErrCanceled) {
				logf("error: %v", err)
				mygo.Dialog.Error(locales.CaptureFailed(), err.Error())
			}
			logf("capture done")
			// The capture, its frames and the image they show are large and
			// done with: what their memory was, the system gets back now
			// rather than at the pace its collector chooses.
			debug.FreeOSMemory()
		}()
	}
	prefs := settings.Load()
	// tell shows the balloon that says where the app is and what starts a
	// capture, for a click that would otherwise seem to do nothing.
	tell := func() {
		note := mygo.NewNotification(mygo.NotificationOptions{
			ID:    "tray",
			Title: locales.TrayReadyTitle(),
			Body:  locales.TrayReadyBody(settings.ShortcutLabel(prefs.Shortcut)),
		})
		go func() {
			if err := note.Show(); err != nil {
				logf("notify: %v", err)
			}
		}()
	}
	apply := func(s settings.Settings) {
		mygo.GlobalShortcut.UnregisterAll()
		if err := mygo.GlobalShortcut.Register(s.Shortcut, capture); err != nil {
			logf("shortcut %s: %v", s.Shortcut, err)
		}
	}
	mygo.App.OnSecondInstance(func(args []string, dir string) {
		// A click on the app again says where the app is, which is where it
		// was: the tray.
		logf("second instance: %v", args)
		tell()
	})
	mygo.App.WhenReady(func() {
		if _, err := mygo.NewTray(mygo.TrayOptions{
			Icon:           trayIcon(),
			IconIsTemplate: trayIsTemplate(),
			ToolTip:        locales.AppName(),
			Menu: mygo.NewMenu([]*mygo.MenuItem{
				{Label: locales.TrayCapture(), Click: func(*mygo.MenuItem, *mygo.Window) { capture() }},
				{Label: locales.SettingsMenu(), Click: func(*mygo.MenuItem, *mygo.Window) { settings.Show(apply) }},
				mygo.Separator(),
				{Role: mygo.RoleQuit},
			}),
		}); err != nil {
			logf("tray: %v", err)
		}
		apply(prefs)
		switch {
		case openedAlone():
			// A click on the app: the tray and the shortcut are the app,
			// which the balloon says.
			tell()
		case o.cmd == "settings":
			settings.Show(apply)
		default:
			capture()
		}
	})
	mygo.App.OnWindowAllClosed(func() {})
	return mygo.App.Run()
}

// shoot captures the screen and delivers what the command asks for.
func shoot(o options) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	img, err := grab(ctx, o)
	if errors.Is(err, capture.ErrDenied) && capture.WaitAllowed(ctx) {
		// The system asked for the permission as the capture failed; take
		// the capture again once the user allows it.
		img, err = capture.Grab(ctx)
	}
	if err != nil {
		return err
	}
	displays := mygo.Screen.Displays()
	if len(displays) == 0 {
		return errors.New("kacha: the desktop reports no display")
	}
	target := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	union := captured(displays, img, target)
	part := region.Place(union, dipRect(target), img.Bounds())
	place := mygo.Rectangle(target.Bounds)

	switch o.cmd {
	case "full":
		return deliver(o, capture.Crop(img, part), overlay.ChoiceBoth)
	case "area":
		crop, err := area(img, part, dipRect(target), o.area)
		if err != nil {
			return err
		}
		return deliver(o, capture.Crop(img, crop), overlay.ChoiceBoth)
	default:
		return overlay.Select(overlay.Options{
			Image:   img,
			Source:  part,
			Place:   place,
			Windows: append(stripsIn(target, part), windowsIn(dipRect(target), part)...),
		}, func(finished image.Image, choice overlay.Choice) error {
			return deliver(o, finished, choice)
		})
	}
}

// grab captures the screen once the delay is over.
func grab(ctx context.Context, o options) (image.Image, error) {
	if o.delay > 0 {
		timer := time.NewTimer(o.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return capture.Grab(ctx)
}

// deliver saves and copies the capture as the flags and the button the user
// finished with ask, and prints the path it wrote.
func deliver(o options, img image.Image, choice overlay.Choice) error {
	save, copy := o.save, o.copy
	if !o.saveSet && o.out != "" {
		save = true
	}
	switch choice {
	case overlay.ChoiceCopyOnly:
		save, copy = false, true
	case overlay.ChoiceSaveOnly:
		save, copy = true, false
	}
	if save {
		path, err := shot.Write(img, o.out)
		if err != nil {
			return err
		}
		fmt.Println(path)
	}
	if copy {
		if err := shot.Copy(img); err != nil {
			logf("copy: %v", err)
			return err
		}
		logf("copy: %dx%d on the clipboard", img.Bounds().Dx(), img.Bounds().Dy())
	}
	return nil
}

// windowsIn returns the windows on d, in the pixels of part, topmost first.
func windowsIn(display region.Rect, part image.Rectangle) []overlay.Window {
	list, err := capture.Windows()
	if err != nil {
		return nil
	}
	if display.Empty() || part.Empty() {
		return nil
	}
	sx := float64(part.Dx()) / display.W
	sy := float64(part.Dy()) / display.H
	var out []overlay.Window
	for _, w := range list {
		r := region.Rect{
			X: float64(w.Rect.Min.X), Y: float64(w.Rect.Min.Y),
			W: float64(w.Rect.Dx()), H: float64(w.Rect.Dy()),
		}.ClampTo(display)
		if r.Empty() {
			continue
		}
		px := image.Rect(
			part.Min.X+int(math.Round((r.X-display.X)*sx)),
			part.Min.Y+int(math.Round((r.Y-display.Y)*sy)),
			part.Min.X+int(math.Round((r.X+r.W-display.X)*sx)),
			part.Min.Y+int(math.Round((r.Y+r.H-display.Y)*sy)),
		).Intersect(part)
		if px.Dx() < 40 || px.Dy() < 40 {
			continue
		}
		out = append(out, overlay.Window{Rect: px, App: w.App, Round: true})
	}
	return out
}

// stripsIn returns the menu bar and the Dock, which the desktop keeps out of
// the work area, as things a click picks.
// stripLabel names the strip of the desktop a click could pick, which goes
// by another name on each system.
func stripLabel(top bool) string {
	switch runtime.GOOS {
	case "windows":
		return locales.StripTaskbar()
	case "darwin":
		if top {
			return locales.StripMenuBar()
		}
		return locales.StripDock()
	}
	if top {
		return locales.StripMenuBar()
	}
	return locales.StripDock()
}

func stripsIn(d mygo.Display, part image.Rectangle) []overlay.Window {
	bounds, work := dipRect(d), workRect(d)
	if bounds.Empty() || part.Empty() {
		return nil
	}
	var out []overlay.Window
	add := func(name string, strip region.Rect) {
		strip = strip.ClampTo(bounds)
		if strip.Empty() {
			return
		}
		px := pixelsOf(strip, bounds, part)
		if px.Dx() < 8 || px.Dy() < 8 {
			return
		}
		out = append(out, overlay.Window{Rect: px, App: name})
	}
	if work.Y > bounds.Y {
		add(stripLabel(true), region.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: work.Y - bounds.Y})
	}
	if work.Y+work.H < bounds.Y+bounds.H {
		add(stripLabel(false), region.Rect{
			X: bounds.X, Y: work.Y + work.H,
			W: bounds.W, H: bounds.Y + bounds.H - work.Y - work.H,
		})
	}
	if work.X > bounds.X {
		add(stripLabel(false), region.Rect{X: bounds.X, Y: bounds.Y, W: work.X - bounds.X, H: bounds.H})
	}
	if work.X+work.W < bounds.X+bounds.W {
		add(stripLabel(false), region.Rect{
			X: work.X + work.W, Y: bounds.Y,
			W: bounds.X + bounds.W - work.X - work.W, H: bounds.H,
		})
	}
	return out
}

// pixelsOf returns r of the screen in the pixels of part.
func pixelsOf(r region.Rect, display region.Rect, part image.Rectangle) image.Rectangle {
	sx := float64(part.Dx()) / display.W
	sy := float64(part.Dy()) / display.H
	return image.Rect(
		part.Min.X+int(math.Round((r.X-display.X)*sx)),
		part.Min.Y+int(math.Round((r.Y-display.Y)*sy)),
		part.Min.X+int(math.Round((r.X+r.W-display.X)*sx)),
		part.Min.Y+int(math.Round((r.Y+r.H-display.Y)*sy)),
	).Intersect(part)
}

// area turns the rectangle of the command line, in DIPs of the display under
// the pointer, into pixels of the capture.
func area(img image.Image, part image.Rectangle, display region.Rect, spec string) (image.Rectangle, error) {
	r, err := region.Area(spec)
	if err != nil {
		return image.Rectangle{}, err
	}
	r.X += display.X
	r.Y += display.Y
	m := region.Mapping{Source: part, Dst: region.Rect{
		X: display.X, Y: display.Y, W: display.W, H: display.H,
	}}
	return m.Pixels(r), nil
}

// captured returns the area of the desktop img covers: the union of the
// displays, or the display under the pointer when the backend captured that
// one alone.
func captured(displays []mygo.Display, img image.Image, target mygo.Display) region.Rect {
	union := region.Union(dipRects(displays)...)
	alone := dipRect(target)
	if union.Empty() || img == nil {
		return union
	}
	want := float64(union.W) / union.H
	got := float64(img.Bounds().Dx()) / float64(img.Bounds().Dy())
	if math.Abs(want-got) > 0.02*want {
		return alone
	}
	return union
}

// workRect returns the work area of a display, in screen DIPs: what a
// window may cover.
func workRect(d mygo.Display) region.Rect {
	w := d.WorkArea
	if w.Width < 1 || w.Height < 1 {
		return dipRect(d)
	}
	return region.Rect{
		X: float64(w.X), Y: float64(w.Y),
		W: float64(w.Width), H: float64(w.Height),
	}
}

// dipRect returns the bounds of a display in screen DIPs.
func dipRect(d mygo.Display) region.Rect {
	return region.Rect{
		X: float64(d.Bounds.X),
		Y: float64(d.Bounds.Y),
		W: float64(d.Bounds.Width),
		H: float64(d.Bounds.Height),
	}
}

// dipRects returns the bounds of every display in screen DIPs.
func dipRects(displays []mygo.Display) []region.Rect {
	out := make([]region.Rect, 0, len(displays))
	for _, d := range displays {
		out = append(out, dipRect(d))
	}
	return out
}
