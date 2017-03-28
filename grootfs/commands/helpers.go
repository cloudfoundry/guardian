package commands

import (
	"fmt"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store/filesystems/btrfs"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/grootfs/store/manager"
	errorspkg "github.com/pkg/errors"
)

type fileSystemDriver interface {
	image_cloner.ImageDriver
	base_image_puller.VolumeDriver
	manager.StoreDriver
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

func parseIDMappings(args []string) ([]groot.IDMappingSpec, error) {
	mappings := []groot.IDMappingSpec{}

	for _, v := range args {
		var mapping groot.IDMappingSpec
		_, err := fmt.Sscanf(v, "%d:%d:%d", &mapping.NamespaceID, &mapping.HostID, &mapping.Size)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}
