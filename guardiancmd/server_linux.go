//go:build linux

package guardiancmd

import (
	"io"
	"os"
)

func mustOpen(path string) io.ReadCloser {
	if r, err := os.Open(path); err != nil {
		panic(err)
	} else {
		return r
	}
}
