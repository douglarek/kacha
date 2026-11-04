//go:build (!linux && !darwin && !windows) || (linux && !(amd64 || arm64))

package capture

import "fmt"

// Detect reports that the platform captures nothing.
func Detect() (Backend, error) {
	return nil, fmt.Errorf("%w: this platform is not supported", ErrUnsupported)
}
