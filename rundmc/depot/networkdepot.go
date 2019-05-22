package depot

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . BindMountSourceCreator
type BindMountSourceCreator interface {
	Create(containerDir string, privileged bool) ([]garden.BindMount, error)
}

//go:generate counterfeiter . RootfsFileCreator
type RootfsFileCreator interface {
	CreateFiles(rootFSPath string, pathsToCreate ...string) error
}

type NetworkDepot struct {
	dir                    string
	rootfsFileCreator      RootfsFileCreator
	bindMountSourceCreator BindMountSourceCreator
}

func NewNetworkDepot(
	dir string,
	rootfsFileCreator RootfsFileCreator,
	bindMountSourceCreator BindMountSourceCreator,
) *NetworkDepot {
	return &NetworkDepot{
		dir:                    dir,
		rootfsFileCreator:      rootfsFileCreator,
		bindMountSourceCreator: bindMountSourceCreator,
	}
}

func (d *NetworkDepot) SetupBindMounts(log lager.Logger, handle string, privileged bool, rootfsPath string) ([]garden.BindMount, error) {
	log = log.Session("network-depot-setup-bindmounts", lager.Data{"handle": handle})

	log.Info("start")
	defer log.Info("finished")

	if err := d.rootfsFileCreator.CreateFiles(rootfsPath, "/etc/hosts", "/etc/resolv.conf"); err != nil {
		log.Error("create-rootfs-mountpoint-files-failed", err)
		return nil, err
	}

	containerDir := d.toDir(handle)
	if err := os.MkdirAll(containerDir, 0755); err != nil {
		log.Error("mkdir-failed", err, lager.Data{"path": containerDir})
		return nil, err
	}

	errs := func(msg string, err error) error {
		removeOrLog(log, containerDir)
		log.Error(msg, err, lager.Data{"path": containerDir})
		return err
	}

	defaultBindMounts, err := d.bindMountSourceCreator.Create(containerDir, !privileged)
	if err != nil {
		return nil, errs("create-bindmount-sources-failed", err)
	}

	return defaultBindMounts, nil
}

func (d *NetworkDepot) Destroy(log lager.Logger, handle string) error {
	log = log.Session("network-depot-destroy", lager.Data{"handle": handle})

	log.Info("start")
	defer log.Info("finished")

	return os.RemoveAll(d.toDir(handle))
}

func (d *NetworkDepot) toDir(handle string) string {
	return filepath.Join(d.dir, handle)
}
