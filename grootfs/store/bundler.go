package store

import (
	"fmt"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type Bundler struct {
	path string
}

func NewBundler(path string) *Bundler {
	return &Bundler{
		path: path,
	}
}

func (b *Bundler) Bundle(id string) groot.Bundle {
	return NewBundle(path.Join(b.path, BUNDLES_DIR_NAME, id))
}

func (b *Bundler) MakeBundle(logger lager.Logger, id string) (groot.Bundle, error) {
	logger = logger.Session("making-bundle", lager.Data{"storePath": b.path, "id": id})
	logger.Info("start")
	defer logger.Info("end")

	bundle := b.Bundle(id)
	if _, err := os.Stat(bundle.Path()); err == nil {
		return nil, fmt.Errorf("bundle for id `%s` already exists", id)
	}

	if err := os.Mkdir(bundle.Path(), 0700); err != nil {
		return nil, fmt.Errorf("making bundle path: %s", err)
	}

	return bundle, nil
}

func (b *Bundler) DeleteBundle(logger lager.Logger, id string) error {
	logger = logger.Session("delete-bundle", lager.Data{"storePath": b.path, "id": id})
	logger.Info("start")
	defer logger.Info("end")

	bundle := b.Bundle(id)
	if _, err := os.Stat(bundle.Path()); err != nil {
		return fmt.Errorf("bundle path not found: %s", err)
	}

	if err := os.RemoveAll(bundle.Path()); err != nil {
		return fmt.Errorf("deleting bundle path: %s", err)
	}

	return nil
}
