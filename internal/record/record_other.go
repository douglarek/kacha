//go:build !darwin && !linux

package record

// Supported reports that this system films no screen for the app.
func Supported() bool { return false }

// Start reports that nothing here films the screen: macOS has a recorder of
// its own that kacha drives, and no other system does.
func Start(Options) (*Recorder, error) { return nil, ErrUnsupported }
