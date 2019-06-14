package depot

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
)

var ErrDoesNotExist = errors.New("does not exist")

//go:generate counterfeiter . BundleSaver
type BundleSaver interface {
	Save(bundle goci.Bndl, path string) error
}

//go:generate counterfeiter . BundleLoader
type BundleLoader interface {
	Load(path string) (goci.Bndl, error)
}

//go:generate counterfeiter . BundleLookupper
type BundleLookupper interface {
	Lookup(log lager.Logger, handle string) (string, error)
}

// a depot which stores containers as subdirs of a depot directory
type DirectoryDepot struct {
	dir          string
	bundleSaver  BundleSaver
	bundleLoader BundleLoader
}

func New(dir string, bundleSaver BundleSaver, bundleLoader BundleLoader) *DirectoryDepot {
	return &DirectoryDepot{
		dir:          dir,
		bundleSaver:  bundleSaver,
		bundleLoader: bundleLoader,
	}
}

func (d *DirectoryDepot) Create(log lager.Logger, handle string, bundle goci.Bndl) (string, error) {
	log = log.Session("depot-create", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	containerDir := d.toDir(handle)
	if err := os.MkdirAll(containerDir, 0755); err != nil {
		log.Error("mkdir-failed", err, lager.Data{"path": containerDir})
		return "", err
	}

	errs := func(msg string, err error) error {
		removeOrLog(log, containerDir)
		log.Error(msg, err, lager.Data{"path": containerDir})
		return err
	}

	if err := d.bundleSaver.Save(bundle, containerDir); err != nil {
		return "", errs("create-failed", err)
	}

	return containerDir, nil
}

func (d *DirectoryDepot) CreatedTime(log lager.Logger, handle string) (time.Time, error) {
	dir, err := d.Lookup(log, handle)
	if err != nil {
		return time.Time{}, err
	}

	info, err := os.Stat(filepath.Join(dir, "pidfile"))
	if err != nil {
		return time.Time{}, fmt.Errorf("bundle pidfile does not exist: %#v", err)
	}

	return info.ModTime(), nil
}

func (d *DirectoryDepot) Load(log lager.Logger, handle string) (goci.Bndl, error) {
	log = log.Session("lood", lager.Data{"handle": handle})

	log.Debug("started")
	defer log.Debug("finished")

	bundlePath, err := d.Lookup(log, handle)
	if err != nil {
		return goci.Bndl{}, err
	}
	return d.bundleLoader.Load(bundlePath)
}

func (d *DirectoryDepot) Lookup(log lager.Logger, handle string) (string, error) {
	log = log.Session("lookup", lager.Data{"handle": handle})

	log.Debug("started")
	defer log.Debug("finished")

	if _, err := os.Stat(d.toDir(handle)); err != nil {
		return "", ErrDoesNotExist
	}

	return d.toDir(handle), nil
}

func (d *DirectoryDepot) Destroy(log lager.Logger, handle string) error {
	log = log.Session("destroy", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	return os.RemoveAll(d.toDir(handle))
}

func (d *DirectoryDepot) GetDir() string {
	return d.dir
}

func (d *DirectoryDepot) Handles() ([]string, error) {
	handles := []string{}
	fileInfos, err := ioutil.ReadDir(d.dir)
	if err != nil {
		return handles, fmt.Errorf("invalid depot directory %s: %s", d.dir, err)
	}

	for _, f := range fileInfos {
		handles = append(handles, f.Name())
	}
	return handles, nil
}

func (d *DirectoryDepot) toDir(handle string) string {
	return filepath.Join(d.dir, handle)
}

func removeOrLog(log lager.Logger, path string) {
	if err := os.RemoveAll(path); err != nil {
		log.Error("remove-failed", err, lager.Data{"path": path})
	}
}
