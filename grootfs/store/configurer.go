package store // import "code.cloudfoundry.org/grootfs/store"

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/lager"
)

func ConfigureStore(logger lager.Logger, storePath string, ownerUID, ownerGID int, imageIDOrPath string) error {
	var data lager.Data
	if imageIDOrPath != "" {
		_, id := filepath.Split(imageIDOrPath)
		data = lager.Data{"id": id}
	}

	if err := ensure(logger, storePath, ownerUID, ownerGID); err != nil {
		logger.Error("failed-to-setup-store", err, data)
		return err
	}

	return nil
}

func ensure(logger lager.Logger, storePath string, ownerUID, ownerGID int) error {
	logger = logger.Session("ensuring-store", lager.Data{"storePath": storePath})
	logger.Debug("start")
	defer logger.Debug("end")

	requiredPaths := []string{
		storePath,
		filepath.Join(storePath, ImageDirName),
		filepath.Join(storePath, VolumesDirName),
		filepath.Join(storePath, CacheDirName),
		filepath.Join(storePath, LocksDirName),
		filepath.Join(storePath, MetaDirName),
		filepath.Join(storePath, TempDirName),
		filepath.Join(storePath, MetaDirName, "dependencies"),
	}

	if err := os.Setenv("TMPDIR", filepath.Join(storePath, TempDirName)); err != nil {
		return fmt.Errorf("could not set TMPDIR: %s", err.Error())
	}

	for _, requiredPath := range requiredPaths {
		if info, err := os.Stat(requiredPath); err == nil {
			if !info.IsDir() {
				return fmt.Errorf("path `%s` is not a directory", requiredPath)
			}

			continue
		}

		if err := os.Mkdir(requiredPath, 0755); err != nil {
			dir, err1 := os.Lstat(requiredPath)
			if err1 != nil || !dir.IsDir() {
				return fmt.Errorf("making directory `%s`: %s", requiredPath, err)
			}
		}

		if err := os.Chown(requiredPath, ownerUID, ownerGID); err != nil {
			logger.Error("store-ownership-change-failed", err, lager.Data{"target-uid": ownerUID, "target-gid": ownerGID})
			return fmt.Errorf("changing store owner to %d:%d for path %s: %s", ownerUID, ownerGID, requiredPath, err.Error())
		}
	}

	if err := createWhiteoutDevice(logger, ownerUID, ownerGID, storePath); err != nil {
		return err
	}

	if err := validateWhiteoutDevice(filepath.Join(storePath, WhiteoutDevice)); err != nil {
		logger.Error("validating-whiteout-device-failed", err)
		return err
	}

	if err := os.Chown(storePath, ownerUID, ownerGID); err != nil {
		logger.Error("store-ownership-change-failed", err, lager.Data{"target-uid": ownerUID, "target-gid": ownerGID})
		return fmt.Errorf("changing store owner to %d:%d for path %s: %s", ownerUID, ownerGID, storePath, err.Error())
	}

	return os.Chmod(storePath, 0700)
}

func createWhiteoutDevice(logger lager.Logger, ownerUID, ownerGID int, storePath string) error {
	whiteoutDevicePath := filepath.Join(storePath, WhiteoutDevice)
	if _, err := os.Stat(whiteoutDevicePath); os.IsNotExist(err) {
		if err := syscall.Mknod(whiteoutDevicePath, syscall.S_IFCHR, 0); err != nil {
			if err != nil && !os.IsExist(err) {
				logger.Error("creating-whiteout-device-failed", err, lager.Data{"path": whiteoutDevicePath})
				return errors.Wrapf(err, "failed to create whiteout device %s", whiteoutDevicePath)
			}
		}

		if err := os.Chown(whiteoutDevicePath, ownerUID, ownerGID); err != nil {
			logger.Error("whiteout-device-ownership-change-failed", err, lager.Data{"target-uid": ownerUID, "target-gid": ownerGID})
			return fmt.Errorf("changing store owner to %d:%d for path %s: %s", ownerUID, ownerGID, whiteoutDevicePath, err.Error())
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
		return fmt.Errorf("the whiteout device file is not a valid device %s", path)
	}

	return nil
}
