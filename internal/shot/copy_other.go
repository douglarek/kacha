//go:build !windows && (!linux || (!amd64 && !arm64))

package shot

import "github.com/egoist/mygo"

// copyPNG puts data on the clipboard as PNG.
func copyPNG(data []byte) error {
	return mygo.Clipboard.WriteImage(data)
}
