//go:build !linux
// +build !linux

package unpacker

import (
	"os"
	"time"
)

func changeModTime(path string, modTime time.Time) error {
	return os.Chtimes(path, time.Now(), modTime)
}
