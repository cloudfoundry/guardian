package commands

import (
	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems/btrfs"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	errorspkg "github.com/pkg/errors"
)

type fileSystemDriver interface {
	image_cloner.ImageDriver
	base_image_puller.VolumeDriver
	store.StoreDriver
}

func createFileSystemDriver(cfg config.Config) (fileSystemDriver, error) {
	switch cfg.FSDriver {
	case "btrfs":
		return btrfs.NewDriver(cfg.BtrfsBin, cfg.DraxBin, cfg.StorePath), nil
	case "overlay-xfs":
		return overlayxfs.NewDriver(cfg.StorePath), nil
	default:
		return nil, errorspkg.Errorf("filesystem driver not supported: %s", cfg.FSDriver)
	}
}
