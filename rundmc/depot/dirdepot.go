package depot

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/cloudfoundry-incubator/guardian/log"
	"github.com/pivotal-golang/lager"
)

var plog = log.Session("depot")

var ErrDoesNotExist = errors.New("does not exist")

//go:generate counterfeiter . BundleSaver
type BundleSaver interface {
	Save(path string) error
}

// a depot which stores containers as subdirs of a depot directory
type DirectoryDepot struct {
	dir string
}

func New(dir string) *DirectoryDepot {
	return &DirectoryDepot{
		dir: dir,
	}
}

func (d *DirectoryDepot) Create(handle string, bundle BundleSaver) error {
	mlog := plog.Start("create", lager.Data{"handle": handle})
	defer mlog.Info("created")

	path := d.toDir(handle)
	if err := os.MkdirAll(path, 0700); err != nil {
		return mlog.Err("mkdir", err, lager.Data{"path": path})
	}

	if err := bundle.Save(path); err != nil {
		mlog.LogIfNotNil("remove-all", os.RemoveAll(path))
		return mlog.Err("create", err, lager.Data{"path": path})
	}

	return nil
}

func (d *DirectoryDepot) Lookup(handle string) (string, error) {
	mlog := plog.Start("lookup", lager.Data{"handle": handle})
	defer mlog.Info("looked-up")

	if _, err := os.Stat(d.toDir(handle)); err != nil {
		return "", ErrDoesNotExist
	}

	return d.toDir(handle), nil
}

func (d *DirectoryDepot) Destroy(handle string) error {
	mlog := plog.Start("destroy", lager.Data{"handle": handle})
	defer mlog.Info("destroyed")

	return os.RemoveAll(d.toDir(handle))
}

func (d *DirectoryDepot) toDir(handle string) string {
	return filepath.Join(d.dir, handle)
}
