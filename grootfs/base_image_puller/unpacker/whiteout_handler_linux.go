//go:build linux
// +build linux

package unpacker

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"github.com/pkg/errors"
)

func (h *overlayWhiteoutHandler) RemoveWhiteout(path string) error {
	toBeDeletedPath := strings.Replace(path, ".wh.", "", 1)
	if err := os.RemoveAll(toBeDeletedPath); err != nil {
		return errors.Wrap(err, "deleting  file")
	}

	targetPath, err := os.Open(filepath.Dir(toBeDeletedPath))
	if err != nil {
		return errors.Wrap(err, "opening target whiteout directory")
	}

	targetName, err := syscall.BytePtrFromString(filepath.Base(toBeDeletedPath))
	if err != nil {
		return errors.Wrap(err, "converting whiteout path to byte pointer")
	}

	whiteoutDevName, err := syscall.BytePtrFromString(overlayxfs.WhiteoutDevice)
	if err != nil {
		return errors.Wrap(err, "converting whiteout device name to byte pointer")
	}

	_, _, errno := syscall.Syscall6(syscall.SYS_LINKAT,
		h.storeDir.Fd(),
		uintptr(unsafe.Pointer(whiteoutDevName)),
		targetPath.Fd(),
		uintptr(unsafe.Pointer(targetName)),
		0,
		0,
	)

	if errno != 0 {
		return errors.Wrapf(errno, "failed to create whiteout node: %s", toBeDeletedPath)
	}

	return nil
}
