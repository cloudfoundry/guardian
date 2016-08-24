package store

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
		filepath.Join(storePath, BUNDLES_DIR_NAME),
		filepath.Join(storePath, VOLUMES_DIR_NAME),
		filepath.Join(storePath, "cache"),
		filepath.Join(storePath, "cache", "blobs"),
	}

	for _, requiredPath := range requiredPaths {
		if info, err := os.Stat(requiredPath); err == nil {
			if !info.IsDir() {
				return fmt.Errorf("path `%s` is not a directory", requiredPath)
			}

			continue
		}

		if err := os.Mkdir(requiredPath, 0700); err != nil {
			return fmt.Errorf("making directory `%s`: %s", requiredPath, err)
		}
	}

	return nil
}
