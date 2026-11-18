package settings

import (
	"runtime"
	"strings"
	"sync"

	"github.com/douglarek/kacha/internal/macwin"
	"github.com/douglarek/kacha/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
)

// Logf, when set, records what the settings could not do.
var Logf = func(format string, any ...any) {}

// stopShortcut releases the global shortcut while the user picks another,
// so that its keys reach the window.
var stopShortcut = func() { mygo.GlobalShortcut.UnregisterAll() }

// syncLoginItem puts the login item where the settings say, which the tests
// leave to the system they run on.
var syncLoginItem = SyncLoginItem

var (
	mu    sync.Mutex
	win   *mygo.Window
	value Settings
	// picking is the shortcut the field takes keys for, "capture" or "record",
	// and empty while it takes none.
	picking   string
	stopWatch func()
	onApply   func(Settings)
)

// Show opens the window of the settings, or brings the open one forward.
// apply runs with the new settings whenever the user changes one.
func Show(apply func(Settings)) {
	mu.Lock()
	defer mu.Unlock()
	onApply = apply
	if win != nil && !win.IsDestroyed() {
		win.Show()
		win.Focus()
		macwin.Activate(win)
		return
	}
	value = Load()
	// The user may have turned the login item off in System Settings.
	value.LaunchAtLogin = LaunchAtLogin()
	picking = ""
	win = mygo.NewWindow(mygo.WindowOptions{
		Title:   locales.SettingsTitle(),
		Width:   460,
		Height:  324,
		Content: ui.View(view),
	})
	win.OnClosed(func() {
		// A window closed while its shortcut was being picked leaves the
		// app without one; it gets the one it has back.
		if picking != "" {
			cancelRecording()
		}
	})
	win.Show()
	win.Focus()
	// A window of an app in the background takes no keys until the app is
	// in front and the window is the key one.
	macwin.Activate(win)
}

// view draws the settings. The keys of a shortcut being picked come from
// WatchKeys, which the app itself cannot take before the field does.
func view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(20).Children(func() {
		ui.Column(c).Background(t.Surface).Radius(12).Border(1, t.Border).Children(func() {
			row(c, t, locales.SettingsLaunchAtLogin(), locales.SettingsLaunchAtLoginHint(), func() {
				if ui.Switch(c, &value.LaunchAtLogin).Changed() {
					save()
				}
			})
			divider(c, t)
			hint := locales.SettingsShortcutHint()
			if picking == "capture" {
				hint = locales.SettingsPressKeysHint()
			}
			row(c, t, locales.SettingsShortcut(), hint, func() {
				shortcutField(c, t, "capture")
			})
			divider(c, t)
			hint = locales.SettingsRecordShortcutHint()
			if picking == "record" {
				hint = locales.SettingsPressKeysHint()
			}
			row(c, t, locales.SettingsRecordShortcut(), hint, func() {
				shortcutField(c, t, "record")
			})
			divider(c, t)
			row(c, t, locales.SettingsCheckUpdates(), locales.SettingsCheckUpdatesHint(), func() {
				ui.Button(c, locales.SettingsCheckUpdatesButton()).OnClick(updater.CheckForUpdates)
			})
		})
	})
}

// row draws a line of the settings: what it is and says, and its control.
func row(c *ui.Context, t *ui.Theme, title, hint string, control func()) {
	ui.Row(c).Padding(16, 13).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			ui.Text(c, title).FontSize(13)
			ui.Text(c, hint).FontSize(11).TextColor(t.TextMuted)
		})
		control()
	})
}

// divider draws the line between the rows.
func divider(c *ui.Context, t *ui.Theme) {
	ui.Box(c).FillWidth().Height(1).Background(t.Border)
}

// shortcutField draws the shortcut, or the empty field that waits for a new
// one, which a click empties and the keys fill.
func shortcutField(c *ui.Context, t *ui.Theme, which string) {
	if picking == which {
		field := ui.Box(c).Width(fieldWidth()).Height(fieldHeight).
			Background(t.Background).Border(2, t.Accent).Radius(7)
		field.Children(func() {
			ui.Row(c).Fill().Justify(ui.Center).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.SettingsPressKeys()).FontSize(10).TextColor(t.TextMuted)
			})
			// The corner gives up the new shortcut, as Escape does.
			ui.Box(c).Absolute().Top(-7).Right(-7).Width(18).Height(18).Radius(9).
				Background(t.Surface).Border(1, t.Border).
				OnClick(cancelRecording).Tooltip(locales.SettingsCancelShortcut()).Children(func() {
				ui.Row(c).Fill().Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "×").FontSize(11).TextColor(t.TextMuted)
				})
			})
		})
		return
	}
	ui.Button(c, shortcutKeys(shortcutOf(which))).Width(fieldWidth()).OnClick(func() { startRecording(which) })
}

// The field of the shortcut is as large as the button of the shortcut, and
// as wide as the platform writes a shortcut: Ctrl+Shift+S takes more room
// than ⌘⇧S.
const fieldHeight = 30

// fieldWidth is the width the shortcut of this platform fits in.
func fieldWidth() float32 {
	if runtime.GOOS == "darwin" {
		return 96
	}
	return 148
}

// startRecording empties the field of the shortcut and takes the next
// combination pressed, which the app does not hold as a global one until
// then.
func startRecording(which string) {
	picking = which
	stopShortcut()
	stopWatch = WatchKeys(func(combo string, escape bool) bool {
		if !pick(combo, escape) {
			return false
		}
		stopWatching()
		redraw()
		return true
	})
}

// stopWatch stops the monitor of the keys, if it runs.
func stopWatching() {
	if stopWatch != nil {
		stopWatch()
		stopWatch = nil
	}
}

// cancelRecording keeps the shortcut there was, as Escape does.
func cancelRecording() {
	pick("", true)
	stopWatching()
	redraw()
}

// pick takes the combination the user pressed, or keeps the one there is for
// an Escape, and reports whether it took the key.
func pick(combo string, escape bool) bool {
	switch {
	case escape:
		picking = ""
		save()
	case valid(combo) && picking != "" && !taken(combo):
		setShortcut(picking, combo)
		picking = ""
		save()
	default:
		return false
	}
	return true
}

// setShortcut puts an accelerator where the shortcut it is for lives.
func setShortcut(which, combo string) {
	if which == "record" {
		value.RecordShortcut = combo
		return
	}
	value.Shortcut = combo
}

// shortcutOf returns the accelerator the settings hold for which.
func shortcutOf(which string) string {
	if which == "record" {
		return value.RecordShortcut
	}
	return value.Shortcut
}

// taken reports whether the other shortcut already holds combo, which one
// key cannot do two things at once for.
func taken(combo string) bool {
	if picking == "record" {
		return combo == value.Shortcut
	}
	return combo == value.RecordShortcut
}

// valid tells whether combo has the modifier a global shortcut needs, and a
// key it may end with.
func valid(combo string) bool {
	return strings.Contains(combo, "+") && !strings.HasSuffix(combo, "+Escape")
}

// redraw draws the window anew, as the settings changed under the keys.
func redraw() {
	if win != nil {
		win.Invalidate()
	}
}

// ShortcutLabel writes an accelerator the way the desktop shows it, which
// the settings and the balloon of the tray say.
func ShortcutLabel(accelerator string) string { return shortcutKeys(accelerator) }

// shortcutKeys writes an accelerator the way a Mac shows it: ⌃⌥⇧⌘S, and the
// way Windows does elsewhere: Ctrl+Shift+S.
func shortcutKeys(accelerator string) string {
	if runtime.GOOS != "darwin" {
		return strings.ReplaceAll(accelerator, "CmdOrCtrl", "Ctrl")
	}
	var mods, key string
	for _, part := range strings.Split(accelerator, "+") {
		switch part {
		case "CmdOrCtrl", "Cmd", "Command":
			mods += "⌘"
		case "Ctrl", "Control":
			mods += "⌃"
		case "Alt", "Option":
			mods += "⌥"
		case "Shift":
			mods += "⇧"
		default:
			key = part
		}
	}
	return mods + key
}

// save writes the settings, and hands them to the app.
func save() {
	if err := Save(value); err != nil {
		Logf("settings: %v", err)
	}
	if err := syncLoginItem(value.LaunchAtLogin); err != nil {
		Logf("settings: %v", err)
	}
	if onApply != nil {
		onApply(value)
	}
}
