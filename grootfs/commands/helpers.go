package commands

import (
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/store/filesystems/btrfs"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	errorspkg "github.com/pkg/errors"
)

func createFileSystemDriver(cfg config.Config) (fileSystemDriver, error) {
	switch cfg.FSDriver {
	case "btrfs":
		return btrfs.NewDriver(cfg.BtrfsBin, cfg.DraxBin, cfg.StorePath)
	case "overlay-xfs":
		return overlayxfs.NewDriver(cfg.StorePath)
	default:
		return nil, errorspkg.Errorf("filesystem driver not supported: %s", cfg.FSDriver)
	}
}
