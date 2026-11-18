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
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/douglarek/kacha/internal/capture"
	"github.com/douglarek/kacha/internal/editor"
	"github.com/douglarek/kacha/internal/overlay"
	"github.com/douglarek/kacha/internal/record"
	"github.com/douglarek/kacha/internal/region"
	"github.com/douglarek/kacha/internal/settings"
	"github.com/douglarek/kacha/internal/shot"
	"github.com/douglarek/kacha/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
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
  kacha record             film the screen, what the drag picks, until stopped
  kacha record --toggle    film until the command runs again, as a key does
  kacha settings           open the settings: the shortcuts and opening at login
  kacha permission         ask macOS for permission to record the screen
  kacha version            print the version

Flags:
  -o, --out PATH   save at PATH, a file or a directory, and turn saving on
                   (default $KACHA_DIR, else ~/Pictures/Screenshots)
  --stdout         write the PNG to stdout instead of saving and copying
  --copy=BOOL      copy the capture to the clipboard (default true)
  --save=BOOL      write the capture to the disk (default false)
  --delay D        wait D before capturing, as 1.5s
  --area SPEC      the rectangle of the area and record commands
  --seconds N      stop a recording after N seconds (default: until stopped)
  --toggle         start a recording, or stop the one a first run started
  --clicks=BOOL    mark the pointer's clicks in a recording (default true)

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
	seconds int
	clicks  bool
	// toggle is a film the key that runs the command again stops, and
	// detached is the film that key started, which runs in a session of its
	// own.
	toggle   bool
	detached bool
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
	record.Logf = logf
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
	fs.IntVar(&o.seconds, "seconds", 0, "stop a recording after this long")
	fs.BoolVar(&o.clicks, "clicks", true, "mark the pointer's clicks in a recording")
	fs.BoolVar(&o.toggle, "toggle", false, "start a recording, or stop the one running")
	fs.BoolVar(&o.detached, "detached", false, "the recording a toggle started")
	if err := fs.Parse(args); err != nil {
		return err
	}
	fs.Visit(func(f *flag.Flag) {
		o.saveSet = o.saveSet || f.Name == "save"
	})
	switch o.cmd {
	case "region", "full", "record":
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
	if o.toggle {
		return toggleRecording(o)
	}
	return gui(o)
}

// docked reports whether this copy runs as the app of a Dock, rather than
// from a terminal, where it takes one capture and leaves.
// openedAlone reports whether a desktop opened the app, with no arguments at
// all, which is how a click on it, or a login, starts it: the app then stays
// in the tray rather than taking a capture no one asked for.
func openedAlone() bool { return openedByDesktop(runtime.GOOS, os.Args[1:], docked()) }

// openedByDesktop tells whether a launch of goos, whose arguments are args,
// is a desktop opening the app: Windows opens one with no arguments, and
// macOS with the process number it gives it, of a copy it knows as a bundle.
func openedByDesktop(goos string, args []string, bundle bool) bool {
	switch goos {
	case "windows":
		return len(args) == 0
	case "darwin":
		if !bundle {
			// A capture asked for from a shell takes one, as it always has.
			return false
		}
		for _, arg := range args {
			// The desktop numbers the process as it opens an app, which is
			// no command of the user's.
			if strings.HasPrefix(arg, "-psn_") {
				continue
			}
			return false
		}
		return true
	}
	return false
}

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

// filmGlyph draws the icon of a film in the tray: the red dot a recorder
// shows while it records, with the square of a stop button in it, which says
// what a click does, ringed so that the dot reads on the light panel and on
// the dark one. A panel shows it at the 16 points of an icon, which 32 pixels
// fill on a display of two pixels a point.
func filmGlyph() []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const dot, ring = 13.8, 15.4
	red := color.NRGBA{R: 0xff, G: 0x3b, B: 0x30, A: 0xff}
	rim := color.NRGBA{R: 0x18, G: 0x18, B: 0x1c, A: 0x88}
	centre := float64(size) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := math.Hypot(float64(x)+0.5-centre, float64(y)+0.5-centre)
			// The edge of the dot covers the pixel as much of it as lies
			// inside, which leaves the circle round at any size.
			if a := clamp(float64(dot) + 0.5 - d); a > 0 {
				img.SetNRGBA(x, y, scale(red, a))
				continue
			}
			if a := clamp(float64(ring) + 0.5 - d); a > 0 {
				img.SetNRGBA(x, y, scale(rim, a))
			}
		}
	}
	// The square of the stop button sits in the middle of the dot, as wide
	// as the dot leaves it room.
	const side = 12
	left, top := centre-side/2, centre-side/2
	white := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			wx := clamp(math.Min(float64(x)+1, left+side) - math.Max(float64(x), left))
			wy := clamp(math.Min(float64(y)+1, top+side) - math.Max(float64(y), top))
			if a := wx * wy; a > 0 {
				img.SetNRGBA(x, y, mix(red, white, a))
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		logf("tray icon: %v", err)
		return nil
	}
	return buf.Bytes()
}

// mix returns a at a of the way to b, which draws one over the other where
// both cover the pixel as much as a.
func mix(a, b color.NRGBA, alpha float64) color.NRGBA {
	at := func(x, y uint8) uint8 { return uint8(float64(x)*(1-alpha) + float64(y)*alpha) }
	return color.NRGBA{R: at(a.R, b.R), G: at(a.G, b.G), B: at(a.B, b.B), A: at(a.A, b.A)}
}

// clamp keeps the coverage of a pixel between none of it and all of it.
func clamp(a float64) float64 { return math.Min(math.Max(a, 0), 1) }

// scale returns c with its alpha as far as alpha takes it.
func scale(c color.NRGBA, alpha float64) color.NRGBA {
	c.A = uint8(float64(c.A) * alpha)
	return c
}

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
// useUpdates adds the update window, which checks, downloads and installs the
// versions `mygo build` signs. It opens only when the user asks for it, from
// the settings or the tray: kacha runs for a capture and quits, so no check
// has a launch to run in.
func useUpdates() {
	mygo.Use(native.New(updater.Options{DisableAutomaticChecks: true}))
}

func gui(o options) error {
	useUpdates()
	// The portal names the app in the picker it shows for a film, by the id
	// the app registered before its first call: register here, before the
	// capture of the overlay, which is a call of the portal already.
	record.Register(context.Background())
	overlay.Logf = logf
	record.Logf = logf
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
	watchFilmNote()
	if o.cmd == "record" {
		// MyGo quits on an interrupt, and a film must say where it went
		// before it goes: the film hears the key itself, and quits the app
		// once the film is written.
		signal.Ignore(os.Interrupt)
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
			take := shoot
			if o.cmd == "record" {
				take = recordScreen
			}
			err := take(o)
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
	mygo.App.OnSecondInstance(func(args []string, dir string) {
		// A click on the app again says where the app is, which is where it
		// was: the tray.
		logf("second instance: %v", args)
		tell()
	})
	// The film being made, if any, and the menu bar item that makes one and
	// stops it: a film runs until the menu stops it, or the app quits.
	var (
		filmMu       sync.Mutex
		filmNow      *record.Recorder
		bar          *mygo.Tray
		startRecord  func()
		stopRecord   func()
		toggleRecord func()
		shortcuts    func()
		apply        func(settings.Settings)
		// offClick drops the listener the icon has while a film is made.
		offClick func()
	)
	filming := func() *record.Recorder {
		filmMu.Lock()
		defer filmMu.Unlock()
		return filmNow
	}
	menuFor := func(filming bool) *mygo.Menu {
		items := []*mygo.MenuItem{
			{Label: locales.TrayCapture(), Click: func(*mygo.MenuItem, *mygo.Window) { capture() }},
		}
		if filming {
			items = append(items, &mygo.MenuItem{Label: locales.TrayStopRecord(), Click: func(*mygo.MenuItem, *mygo.Window) { stopRecord() }})
		} else {
			items = append(items, &mygo.MenuItem{Label: locales.TrayRecord(), Click: func(*mygo.MenuItem, *mygo.Window) { startRecord() }})
		}
		items = append(items,
			&mygo.MenuItem{Label: locales.SettingsMenu(), Click: func(*mygo.MenuItem, *mygo.Window) { settings.Show(apply) }},
			updater.MenuItem(),
			mygo.Separator(),
			&mygo.MenuItem{Role: mygo.RoleQuit},
		)
		return mygo.NewMenu(items)
	}
	showFilm := func(rec *record.Recorder) {
		filmMu.Lock()
		filmNow = rec
		filmMu.Unlock()
		if bar == nil {
			return
		}
		if offClick != nil {
			offClick()
			offClick = nil
		}
		if rec == nil {
			bar.SetTitle("")
			bar.SetToolTip(locales.AppName())
			bar.SetMenu(menuFor(false))
			return
		}
		bar.SetToolTip(locales.TrayRecordingTip())
		if runtime.GOOS != "darwin" {
			// The panels of the other systems open the menu on a click, and
			// one without items would leave the film with no way out of the
			// menu: a film stops by its item there.
			bar.SetMenu(menuFor(true))
			return
		}
		// A film stops from the icon itself: a menu takes the click that
		// would stop it, and shows in the film it stops.
		bar.SetMenu(nil)
		offClick = bar.OnClick(stopRecord)
		go func() {
			began := time.Now()
			for {
				select {
				case <-rec.Done():
					return
				case <-time.After(time.Second):
					passed := time.Since(began)
					bar.SetTitle(fmt.Sprintf("● %d:%02d", int(passed.Minutes()), int(passed.Seconds())%60))
				}
			}
		}()
	}
	// shortcuts registers what the keys do now: a capture takes one, and a
	// film takes the other, which starts a film and ends it.
	shortcuts = func() {
		mygo.GlobalShortcut.UnregisterAll()
		if err := mygo.GlobalShortcut.Register(prefs.Shortcut, capture); err != nil {
			logf("shortcut %s: %v", prefs.Shortcut, err)
		}
		if prefs.RecordShortcut == "" {
			return
		}
		if err := mygo.GlobalShortcut.Register(prefs.RecordShortcut, toggleRecord); err != nil {
			logf("shortcut %s to record: %v", prefs.RecordShortcut, err)
		}
	}
	startRecord = func() {
		if filming() != nil || !busy.CompareAndSwap(false, true) {
			return
		}
		go func() {
			defer busy.Store(false)
			defer func() {
				if r := recover(); r != nil {
					logf("panic: %v\n%s", r, debug.Stack())
					mygo.Dialog.Error(locales.RecordFailed(), fmt.Sprint(r))
				}
			}()
			logf("record start")
			rec, err := film(options{cmd: "record", clicks: true})
			if err != nil {
				if !errors.Is(err, overlay.ErrCanceled) {
					logf("record: %v", err)
					mygo.Dialog.Error(locales.RecordFailed(), err.Error())
				}
				return
			}
			showFilm(rec)
			shortcuts()
			path, err := rec.Wait()
			showFilm(nil)
			shortcuts()
			if err != nil {
				logf("record: %v", err)
				mygo.Dialog.Error(locales.RecordFailed(), err.Error())
				return
			}
			logf("record done: %s", path)
			filmSaved(path)
			debug.FreeOSMemory()
		}()
	}
	stopRecord = func() {
		if rec := filming(); rec != nil {
			go func() { _, _ = rec.Stop() }()
		}
	}
	toggleRecord = func() {
		if filming() != nil {
			stopRecord()
			return
		}
		startRecord()
	}
	apply = func(s settings.Settings) {
		prefs = s
		shortcuts()
	}
	mygo.App.WhenReady(func() {
		created, err := mygo.NewTray(mygo.TrayOptions{
			Icon:           trayIcon(),
			IconIsTemplate: trayIsTemplate(),
			ToolTip:        locales.AppName(),
			Menu:           menuFor(false),
		})
		if err != nil {
			logf("tray: %v", err)
		}
		bar = created
		apply(prefs)
		switch {
		case openedAlone():
			// A click on the app: the tray and the shortcut are the app,
			// which the balloon says.
			tell()
		case o.cmd == "settings":
			settings.Show(apply)
		case o.cmd == "record":
			// A film the app takes starts from its menu, which also stops it.
			startRecord()
		default:
			capture()
		}
	})
	mygo.App.OnQuit(func() {
		// A film goes on in screencapture, which would outlive the app: the
		// quit stops it, and the file is written as a stop would.
		if rec := filming(); rec != nil {
			_, _ = rec.Stop()
		}
	})
	mygo.App.OnWindowAllClosed(func() {})
	watchFilmNote()
	return mygo.App.Run()
}

// reveal shows the file a film went to in the Finder, which is where the
// user looks for it.
func reveal(path string) {
	var err error
	switch runtime.GOOS {
	case "darwin":
		err = exec.Command("open", "-R", path).Run()
	case "linux":
		err = showInFileManager(path)
	default:
		return
	}
	if err != nil {
		logf("reveal: %v", err)
	}
}

// showInFileManager asks the file manager of the session to show path, which
// selects it where the desktop answers org.freedesktop.FileManager1, and
// opens the folder it is in where it does not.
func showInFileManager(path string) error {
	if err := exec.Command("gdbus", showItemsArgs(path)...).Run(); err == nil {
		return nil
	}
	return exec.Command("xdg-open", filepath.Dir(path)).Run()
}

// showItemsArgs returns the call of org.freedesktop.FileManager1.ShowItems
// that shows path: the file managers read it as the file to select.
func showItemsArgs(path string) []string {
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	return []string{
		"call", "--session",
		"--dest", "org.freedesktop.FileManager1",
		"--object-path", "/org/freedesktop/FileManager1",
		"--method", "org.freedesktop.FileManager1.ShowItems",
		"[" + strconv.Quote(uri) + "]", "",
	}
}

// scene captures the screen and returns what a window over it shows: the
// image, the part of it the window shows, where the window goes, and what a
// click on it could pick.
func scene(o options) (image.Image, image.Rectangle, mygo.Rectangle, []overlay.Window, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	img, err := grab(ctx, o)
	if errors.Is(err, capture.ErrDenied) && capture.WaitAllowed(ctx) {
		// The system asked for the permission as the capture failed; take
		// the capture again once the user allows it.
		img, err = capture.Grab(ctx)
	}
	if err != nil {
		return nil, image.Rectangle{}, mygo.Rectangle{}, nil, err
	}
	displays := mygo.Screen.Displays()
	if len(displays) == 0 {
		return nil, image.Rectangle{}, mygo.Rectangle{}, nil, errors.New("kacha: the desktop reports no display")
	}
	target := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	union := captured(displays, img, target)
	part := region.Place(union, dipRect(target), img.Bounds())
	place := mygo.Rectangle(target.Bounds)
	windows := append(stripsIn(target, part), windowsIn(dipRect(target), part)...)
	return img, part, place, windows, nil
}

// shoot captures the screen and delivers what the command asks for.
func shoot(o options) error {
	img, part, place, windows, err := scene(o)
	if err != nil {
		return err
	}
	display := region.Rect{
		X: float64(place.X), Y: float64(place.Y),
		W: float64(place.Width), H: float64(place.Height),
	}
	switch o.cmd {
	case "full":
		return deliver(o, capture.Crop(img, part), overlay.ChoiceBoth)
	case "area":
		crop, err := area(img, part, display, o.area)
		if err != nil {
			return err
		}
		return deliver(o, capture.Crop(img, crop), overlay.ChoiceBoth)
	default:
		return overlay.Select(overlay.Options{
			Image:   img,
			Source:  part,
			Place:   place,
			Windows: windows,
		}, func(finished image.Image, choice overlay.Choice) error {
			return deliver(o, finished, choice)
		})
	}
}

// recordScreen films the screen, what --area names or what the user drags,
// until its time is up or the command is interrupted, and prints the file.
func recordScreen(o options) error {
	if o.detached {
		// A film a key started has no terminal to say where it went: it
		// leaves the file to the key that stops it, and says both in a
		// balloon.
		defer record.Unmark()
	}
	// The key that started the film stops it with SIGUSR1, which no app quits
	// on: an interrupt quits mygo's event loop, and the notification that
	// says where the film went is no notification then. It is heard from
	// here, before the picker of the desktop asks what to film, where a
	// press would end the process otherwise.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, stopSignals...)
	defer signal.Stop(stop)

	// The tray goes up before the film does, as the area the user picks and
	// the screen the desktop shares hold a film back: its click ends the
	// film, or the launch that has not started one yet, and a film a
	// terminal made has it too, which says it records where a terminal
	// cannot.
	fromTray := make(chan struct{}, 1)
	var tray *mygo.Tray
	if runtime.GOOS == "linux" {
		tray = filmTray(func() {
			logf("record: the tray stops the film")
			select {
			case fromTray <- struct{}{}:
			default:
			}
		})
	}
	defer func() {
		if tray != nil {
			tray.Destroy()
		}
	}()

	rec, stopped, err := beginFilm(o, stop, fromTray)
	if stopped {
		// The key, or the tray, came before the film did: it says so, as a
		// launch that goes quietly looks like one that filmed nothing.
		logf("record: stopped before it filmed")
		notify(locales.RecordStoppedEarly(), "")
		return nil
	}
	if err != nil {
		// A film the desktop turned down says so in a balloon: a launch with
		// no terminal would go quietly, as if it had never asked.
		if errors.Is(err, record.ErrDenied) {
			notify(locales.RecordStoppedEarly(), "")
		}
		return err
	}

	select {
	case <-rec.Done():
		logf("record: the recorder ended by itself")
	case <-stop:
		logf("record: stopping")
	case <-fromTray:
		logf("record: stopping from the tray")
	}
	path, err := rec.Stop()
	if tray != nil {
		tray.Destroy()
		tray = nil
	}
	logf("record: stopped, path=%q err=%v", path, err)
	if err != nil {
		return err
	}
	if o.detached {
		record.Settled(path)
	}
	// The notification says where the film went, and it is shown before the
	// launch goes: an app that quits at once would take it along. A film a
	// terminal made says it there as well, as a launch may have no terminal
	// to read.
	filmSaved(path)
	if !o.detached {
		fmt.Println(path)
	}
	return nil
}

// toggleRecording films until the command runs again, which a key of the
// desktop runs: the film itself goes on in a session of its own, so that the
// key returns at once, and a second run stops it.
func toggleRecording(o options) error {
	if pid, ok := record.Filming(); ok {
		logf("record: toggle stops %d", pid)
		path, err := record.StopFilming(pid)
		logf("record: toggle stopped, path=%q err=%v", path, err)
		if err != nil {
			return err
		}
		if path != "" {
			fmt.Println(path)
		}
		return nil
	}
	if !record.Supported() {
		return record.ErrUnsupported
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("kacha: record: %w", err)
	}
	args := []string{"record", "--detached"}
	if o.out != "" {
		args = append(args, "-o", o.out)
	}
	if o.area != "" {
		args = append(args, "--area", o.area)
	}
	if o.seconds > 0 {
		args = append(args, "--seconds", strconv.Itoa(o.seconds))
	}
	if !o.clicks {
		args = append(args, "--clicks=false")
	}
	pid, err := record.Detach(exe, args)
	logf("record: toggle starts %d: %v", pid, err)
	return err
}

// filmNote is the ID of the notification that says where a film went: a
// click on it opens the folder the film is in.
const filmNote = "film"

// filmPath is where the last film went.
var filmPath struct {
	sync.Mutex
	path string
}

// rememberFilm keeps where a film went, for the click of its notification.
func rememberFilm(path string) {
	filmPath.Lock()
	defer filmPath.Unlock()
	filmPath.path = path
}

// lastFilm returns where the last film went, "" without one.
func lastFilm() string {
	filmPath.Lock()
	defer filmPath.Unlock()
	return filmPath.path
}

// watchFilmNote opens the folder of the last film when its notification is
// clicked. It is registered before the app runs, as a click of a notification
// of an earlier launch arrives then.
func watchFilmNote() {
	mygo.App.OnNotificationClick(func(id string) {
		if id != filmNote {
			return
		}
		path := lastFilm()
		if path == "" {
			return
		}
		logf("film note clicked: %s", path)
		reveal(path)
	})
}

// filmSaved says where a film went, in a notification whose click opens the
// folder it is in. It is shown before the call returns: a launch that goes at
// once, as the tray stopped one does, would take the notification with it.
func filmSaved(path string) {
	rememberFilm(path)
	note := mygo.NewNotification(mygo.NotificationOptions{
		ID:    filmNote,
		Title: locales.RecordSaved(),
		Body:  path,
	})
	if err := note.Show(); err != nil {
		logf("notify: %v", err)
	}
}

// beginFilm starts what o films, and stops waiting for it where the key that
// started the film comes first: the launch goes, and the picker of the
// desktop with it.
func beginFilm(o options, stop <-chan os.Signal, fromTray <-chan struct{}) (rec *record.Recorder, stopped bool, err error) {
	type made struct {
		rec *record.Recorder
		err error
	}
	madeCh := make(chan made, 1)
	go func() {
		r, err := film(o)
		madeCh <- made{r, err}
	}()
	select {
	case <-stop:
		return nil, true, nil
	case <-fromTray:
		return nil, true, nil
	case m := <-madeCh:
		return m.rec, false, m.err
	}
}

// filmTray puts an icon in the tray while a film that a key started runs: a
// click on it ends the film, which the key that started it also does, but a
// button is easier to find than a key.
func filmTray(stop func()) *mygo.Tray {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "film",
		Icon:    filmGlyph(),
		ToolTip: locales.TrayRecordingTip(),
		Menu: mygo.NewMenu([]*mygo.MenuItem{
			{Label: locales.TrayStopRecord(), Click: func(*mygo.MenuItem, *mygo.Window) { stop() }},
		}),
	})
	if err != nil {
		logf("tray: %v", err)
		return nil
	}
	logf("record: the tray of the film is up")
	tray.OnClick(stop)
	return tray
}

// notify shows a balloon, for a launch that has no window to say it in. It is
// shown before the call returns, as a launch that says something and goes at
// once would take the balloon with it.
func notify(title, body string) {
	note := mygo.NewNotification(mygo.NotificationOptions{ID: "record", Title: title, Body: body})
	if err := note.Show(); err != nil {
		logf("notify: %v", err)
	}
}

// film shows what to record, and returns the recording once it is rolling.
func film(o options) (*record.Recorder, error) {
	if !record.Supported() {
		return nil, record.ErrUnsupported
	}
	img, part, place, windows, err := scene(o)
	if err != nil {
		return nil, err
	}
	rect, err := recordRect(o, img, part, place, windows)
	if err != nil {
		return nil, err
	}
	logf("record: filming %v, the screen at %v", rect, place)
	if o.area == "" {
		// The window the drag was made in is on its way out, and a film
		// should not open with it.
		time.Sleep(150 * time.Millisecond)
	}
	return record.Start(record.Options{
		Rect:    rect,
		Display: image.Rect(place.X, place.Y, place.X+place.Width, place.Y+place.Height),
		Seconds: o.seconds,
		Clicks:  o.clicks,
		Out:     o.out,
		// The balloon says the film has begun as the recorder rolls, which
		// the icon of the tray went up for long before: the area, the screen
		// the desktop shares and the codec take their time.
		Started: func() {
			// A film says it began from either launch, and a Wayland session
			// names the icon of the film as the way to end it, which the key
			// that started it no longer is alone.
			if runtime.GOOS == "linux" {
				notify(locales.RecordStartedTray(), "")
				return
			}
			notify(locales.RecordStarted(), "")
		},
	})
}

// recordRect returns what to film, in the DIPs of the screen: the rectangle
// --area names, or what the user drags over the capture.
func recordRect(o options, img image.Image, part image.Rectangle, place mygo.Rectangle, windows []overlay.Window) (image.Rectangle, error) {
	if o.area != "" {
		r, err := region.Area(o.area)
		if err != nil {
			return image.Rectangle{}, err
		}
		return image.Rect(
			place.X+int(r.X), place.Y+int(r.Y),
			place.X+int(r.X+r.W), place.Y+int(r.Y+r.H),
		), nil
	}
	picked, err := overlay.Pick(overlay.Options{
		Image:   img,
		Source:  part,
		Place:   place,
		Hint:    locales.RecordHint(),
		Windows: windows,
	})
	if err != nil {
		return image.Rectangle{}, err
	}
	return image.Rect(
		place.X+int(picked.X), place.Y+int(picked.Y),
		place.X+int(picked.X+picked.W), place.Y+int(picked.Y+picked.H),
	), nil
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
