package store // import "code.cloudfoundry.org/grootfs/store"

import (
	"fmt"
	"os"
	"path/filepath"

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
		filepath.Join(storePath, IMAGES_DIR_NAME),
		filepath.Join(storePath, VOLUMES_DIR_NAME),
		filepath.Join(storePath, CACHE_DIR_NAME),
		filepath.Join(storePath, LOCKS_DIR_NAME),
		filepath.Join(storePath, META_DIR_NAME),
		filepath.Join(storePath, TEMP_DIR_NAME),
		filepath.Join(storePath, META_DIR_NAME, "dependencies"),
	}

	if err := os.Setenv("TMPDIR", filepath.Join(storePath, TEMP_DIR_NAME)); err != nil {
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

	if err := os.Chown(storePath, ownerUID, ownerGID); err != nil {
		logger.Error("store-ownership-change-failed", err, lager.Data{"target-uid": ownerUID, "target-gid": ownerGID})
		return fmt.Errorf("changing store owner to %d:%d for path %s: %s", ownerUID, ownerGID, storePath, err.Error())
	}

	return os.Chmod(storePath, 0700)
}
