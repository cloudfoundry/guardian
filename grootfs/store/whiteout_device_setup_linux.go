package store

import (
	"os"
	"path/filepath"
	"syscall"

	errorspkg "github.com/pkg/errors"

	"code.cloudfoundry.org/lager"
)

func createWhiteoutDevice(logger lager.Logger, ownerUID, ownerGID int, storePath string) error {
	whiteoutDevicePath := filepath.Join(storePath, WhiteoutDevice)
	if _, err := os.Stat(whiteoutDevicePath); os.IsNotExist(err) {
		if err := syscall.Mknod(whiteoutDevicePath, syscall.S_IFCHR, 0); err != nil {
			if err != nil && !os.IsExist(err) {
				logger.Error("creating-whiteout-device-failed", err, lager.Data{"path": whiteoutDevicePath})
				return errorspkg.Wrapf(err, "failed to create whiteout device %s", whiteoutDevicePath)
			}
		}

		if err := os.Chown(whiteoutDevicePath, ownerUID, ownerGID); err != nil {
			logger.Error("whiteout-device-ownership-change-failed", err, lager.Data{"target-uid": ownerUID, "target-gid": ownerGID})
			return errorspkg.Wrapf(err, "changing store owner to %d:%d for path %s", ownerUID, ownerGID, whiteoutDevicePath)
		}
	}
	return nil
}

func validateWhiteoutDevice(path string) error {
	stat, err := os.Stat(path)
	if err != nil && !os.IsExist(err) {
		return err
	}

	statT := stat.Sys().(*syscall.Stat_t)
	if statT.Rdev != 0 || (stat.Mode()&os.ModeCharDevice) != os.ModeCharDevice {
		return errorspkg.Errorf("the whiteout device file is not a valid device %s", path)
	}

	return nil
}
