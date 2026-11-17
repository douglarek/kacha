//go:build !darwin && !linux

package record

// Filming reports no film, where no film is made.
func Filming() (int, bool) { return 0, false }

// StopFilming reports that nothing here films the screen.
func StopFilming(int) (string, error) { return "", ErrUnsupported }

// Detach reports that nothing here films the screen.
func Detach(string, []string) (int, error) { return 0, ErrUnsupported }

// Unmark does nothing here.
func Unmark() {}

// Settled does nothing here.
func Settled(string) {}

// ClearState does nothing here.
func ClearState() {}
