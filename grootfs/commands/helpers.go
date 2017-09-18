package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/store/filesystems/btrfs"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
	"github.com/opencontainers/runc/libcontainer/user"
	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli"
)

type fileSystemDriver interface {
	CreateImage(logger lager.Logger, spec image_cloner.ImageDriverSpec) (groot.MountInfo, error)
	DestroyImage(logger lager.Logger, path string) error
	FetchStats(logger lager.Logger, path string) (groot.VolumeStats, error)
	ConfigureStore(logger lager.Logger, storePath string, ownerUID, ownerGID int) error
	ValidateFileSystem(logger lager.Logger, path string) error
	InitFilesystem(logger lager.Logger, filesystemPath, storePath string) error
	VolumePath(logger lager.Logger, id string) (string, error)
	Volumes(logger lager.Logger) ([]string, error)
	VolumeSize(lager.Logger, string) (int64, error)
	CreateVolume(logger lager.Logger, parentID, id string) (string, error)
	DestroyVolume(logger lager.Logger, id string) error
	MoveVolume(logger lager.Logger, from, to string) error
	WriteVolumeMeta(logger lager.Logger, id string, data base_image_puller.VolumeMeta) error
	HandleOpaqueWhiteouts(logger lager.Logger, id string, opaqueWhiteouts []string) error
	Marshal(logger lager.Logger) ([]byte, error)
}

func createFileSystemDriver(cfg config.Config) (fileSystemDriver, error) {
	switch cfg.FSDriver {
	case "btrfs":
		return btrfs.NewDriver(filepath.Join(cfg.BtrfsProgsPath, "btrfs"),
			filepath.Join(cfg.BtrfsProgsPath, "mkfs.btrfs"), cfg.DraxBin, cfg.StorePath), nil
	case "overlay-xfs":
		return overlayxfs.NewDriver(cfg.StorePath, cfg.TardisBin, cfg.Init.ExternalLogdevSize), nil
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

func readSubUIDMapping(username string) ([]groot.IDMappingSpec, error) {
	user, err := user.LookupUser(username)
	if err != nil {
		return nil, err
	}

	return readSubIDMapping(username, user.Uid, "/etc/subuid")
}

func readSubGIDMapping(groupname string) ([]groot.IDMappingSpec, error) {
	group, err := user.LookupGroup(groupname)
	if err != nil {
		return nil, err
	}

	return readSubIDMapping(groupname, group.Gid, "/etc/subgid")
}

func readSubIDMapping(name string, id int, subidPath string) ([]groot.IDMappingSpec, error) {
	mappings := []groot.IDMappingSpec{{
		HostID: id, NamespaceID: 0, Size: 1,
	}}

	contents, err := ioutil.ReadFile(subidPath)
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Fields(string(contents)) {
		entry := strings.Split(line, ":")
		if entry[0] == name {
			hostID, err := strconv.Atoi(entry[1])
			if err != nil {
				return nil, err
			}
			size, err := strconv.Atoi(entry[2])
			if err != nil {
				return nil, err
			}
			mappings = append(mappings, groot.IDMappingSpec{
				HostID:      hostID,
				NamespaceID: 1,
				Size:        size,
			})
		}
	}

	return mappings, nil
}

type exitErrorFunc func(message string, exitCode int) *cli.ExitError

func newErrorHandler(logger lager.Logger, action string) exitErrorFunc {
	metricsEmitter := metrics.NewEmitter()

	return func(message string, exitCode int) *cli.ExitError {
		err := errors.New(message)
		defer metricsEmitter.TryIncrementRunCount(action, err)
		metricsEmitter.TryEmitError(logger, action, err, int32(exitCode))
		return cli.NewExitError(message, exitCode)
	}
}
