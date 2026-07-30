package groot

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager/v3"
	errorspkg "github.com/pkg/errors"
)

type Lister struct {
}

func IamLister() *Lister {
	return &Lister{}
}

func (l *Lister) List(logger lager.Logger, storePath string) ([]string, error) {
	logger = logger.Session("groot-listing", lager.Data{"storePath": storePath})
	logger.Info("starting")
	defer logger.Info("ending")

	imagePaths, err := l.listDirs(filepath.Join(storePath, store.ImageDirName))
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to list store path")
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
	_ = f.Close()
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
