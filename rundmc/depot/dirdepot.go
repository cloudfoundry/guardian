package depot

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
)

var ErrDoesNotExist = errors.New("does not exist")

//go:generate counterfeiter . BundleSaver
type BundleSaver interface {
	Save(bundle goci.Bndl, path string) error
}

// a depot which stores containers as subdirs of a depot directory
type DirectoryDepot struct {
	dir         string
	bundleSaver BundleSaver
}

func New(dir string, bundleSaver BundleSaver) *DirectoryDepot {
	return &DirectoryDepot{
		dir:         dir,
		bundleSaver: bundleSaver,
	}
}

func (d *DirectoryDepot) Create(log lager.Logger, handle string, bundle goci.Bndl) error {
	log = log.Session("depot-create", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	containerDir := d.toDir(handle)
	if err := os.MkdirAll(containerDir, 0755); err != nil {
		log.Error("mkdir-failed", err, lager.Data{"path": containerDir})
		return err
	}

	if err := touchFile(filepath.Join(containerDir, "hosts")); err != nil {
		return err
	}
	if err := touchFile(filepath.Join(containerDir, "resolv.conf")); err != nil {
		return err
	}

	mounts := []specs.Mount{}
	if _, err := os.Stat(filepath.Join(bundle.RootFS(), "etc", "hosts")); err == nil {
		mounts = append(mounts, specs.Mount{
			Destination: "/etc/hosts",
			Source:      filepath.Join(containerDir, "hosts"),
			Type:        "bind",
			Options:     []string{"bind"},
		})
	}
	if _, err := os.Stat(filepath.Join(bundle.RootFS(), "etc", "resolv.conf")); err == nil {
		mounts = append(mounts, specs.Mount{
			Destination: "/etc/resolv.conf",
			Source:      filepath.Join(containerDir, "resolv.conf"),
			Type:        "bind",
			Options:     []string{"bind"},
		})
	}

	bundle = bundle.WithMounts(mounts...)

	if err := d.bundleSaver.Save(bundle, containerDir); err != nil {
		removeOrLog(log, containerDir)
		log.Error("create-failed", err, lager.Data{"path": containerDir})
		return err
	}

	return nil
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

//go:generate counterfeiter . BundleLoader
type BundleLoader interface {
	Load(bundleDir string) (goci.Bndl, error)
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

func touchFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}
