package groot

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
)

type Lister struct {
}

func IamLister() *Lister {
	return &Lister{}
}

func (l *Lister) List(logger lager.Logger, storePath string) ([]string, error) {
	logger = logger.Session("groot-listing", lager.Data{"storePath": storePath})
	logger.Info("start")
	defer logger.Info("end")

	imagePaths := []string{}
	subStores, err := l.listDirs(storePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list store path: %s", err)
	}
	for _, subStore := range subStores {
		images, err := l.listDirs(filepath.Join(subStore, store.IMAGES_DIR_NAME))
		if err != nil {
			return nil, fmt.Errorf("failed to list substore path: %s", err)
		}

		imagePaths = append(imagePaths, images...)
	}

	logger.Debug("list-images", lager.Data{"imagePaths": imagePaths})
	return imagePaths, nil
}

func (l *Lister) listDirs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	files, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}

	names := []string{}
	for _, fileInfo := range files {
		fullPath := filepath.Join(path, fileInfo.Name())
		names = append(names, fullPath)
	}

	return names, nil
}
