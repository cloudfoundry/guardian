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
) NetworkDepot {
	return NetworkDepot{
		dir:                    dir,
		rootfsFileCreator:      rootfsFileCreator,
		bindMountSourceCreator: bindMountSourceCreator,
	}
}

func (d NetworkDepot) SetupBindMounts(log lager.Logger, handle string, privileged bool, rootfsPath string) ([]garden.BindMount, error) {
	log = log.Session("network-depot-setup-bindmounts", lager.Data{"handle": handle})

	log.Info("start")
	defer log.Info("finished")
	return d.setupBindMounts(log, d.toDir(handle), privileged, rootfsPath)
}

func (d NetworkDepot) setupBindMounts(log lager.Logger, depotDirectory string, privileged bool, rootfsPath string) ([]garden.BindMount, error) {
	if err := d.rootfsFileCreator.CreateFiles(rootfsPath, "/etc/hosts", "/etc/resolv.conf"); err != nil {
		log.Error("create-rootfs-mountpoint-files-failed", err)
		return nil, err
	}

	if err := os.MkdirAll(depotDirectory, 0755); err != nil {
		log.Error("mkdir-failed", err, lager.Data{"depotDirectory": depotDirectory})
		return nil, err
	}

	errs := func(msg string, err error) error {
		d.cleanupOrLog(log, depotDirectory)
		log.Error(msg, err, lager.Data{"depotDirectory": depotDirectory})
		return err
	}

	defaultBindMounts, err := d.bindMountSourceCreator.Create(depotDirectory, !privileged)
	if err != nil {
		return nil, errs("create-bindmount-sources-failed", err)
	}

	return defaultBindMounts, nil
}

func (d NetworkDepot) Destroy(log lager.Logger, handle string) error {
	log = log.Session("network-depot-destroy", lager.Data{"handle": handle})

	log.Info("start")
	defer log.Info("finished")

	return d.cleanup(handle)
}

func (d NetworkDepot) toDir(handle string) string {
	return filepath.Join(d.dir, handle)
}

func (d NetworkDepot) cleanup(containerDir string) error {
	for _, f := range []string{"hosts", "resolv.conf"} {
		if err := os.RemoveAll(filepath.Join(containerDir, f)); err != nil {
			return err
		}
	}

	return nil
}

func (d NetworkDepot) cleanupOrLog(log lager.Logger, depotDir string) {
	if err := d.cleanup(depotDir); err != nil {
		log.Error("cleanup-failed", err, lager.Data{"depotDir": depotDir})
	}
}
