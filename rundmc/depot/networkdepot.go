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

type NetworkDepot struct {
	dir                    string
	bindMountSourceCreator BindMountSourceCreator
}

func NewNetworkDepot(
	dir string,
	bindMountSourceCreator BindMountSourceCreator,
) NetworkDepot {
	return NetworkDepot{
		dir:                    dir,
		bindMountSourceCreator: bindMountSourceCreator,
	}
}

func (d NetworkDepot) SetupBindMounts(log lager.Logger, handle string, privileged bool, rootfsPath string) ([]garden.BindMount, error) {
	log = log.Session("network-depot-setup-bindmounts", lager.Data{"handle": handle})

	log.Info("start")
	defer log.Info("finished")

	containerDir := d.toDir(handle)
	if err := os.MkdirAll(containerDir, 0755); err != nil {
		log.Error("mkdir-failed", err, lager.Data{"path": containerDir})
		return nil, err
	}

	errs := func(msg string, err error) error {
		d.cleanupOrLog(log, handle)
		log.Error(msg, err, lager.Data{"path": containerDir})
		return err
	}

	defaultBindMounts, err := d.bindMountSourceCreator.Create(containerDir, !privileged)
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

func (d NetworkDepot) cleanup(handle string) error {
	containerDir := d.toDir(handle)

	for _, f := range []string{"hosts", "resolv.conf"} {
		if err := os.RemoveAll(filepath.Join(containerDir, f)); err != nil {
			return err
		}
	}

	return nil
}

func (d NetworkDepot) cleanupOrLog(log lager.Logger, handle string) {
	if err := d.cleanup(handle); err != nil {
		log.Error("cleanup-failed", err, lager.Data{"handle": handle})
	}
}
