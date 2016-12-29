package store // import "code.cloudfoundry.org/grootfs/store"

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager"
)

type Configurer struct {
}

func NewConfigurer() *Configurer {
	return &Configurer{}
}

func (c *Configurer) Ensure(logger lager.Logger, storePath string) error {
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
		filepath.Join(storePath, META_DIR_NAME, "dependencies"),
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
	}

	return os.Chmod(storePath, 0700)
}
