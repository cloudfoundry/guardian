package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/commands/commandrunner"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/metrics/systemreporter"
	"code.cloudfoundry.org/grootfs/store/filesystems/btrfs"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/lager"
	"github.com/opencontainers/runc/libcontainer/user"
	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli"
)

type fileSystemDriver interface {
	image_cloner.ImageDriver
	base_image_puller.VolumeDriver
	manager.StoreDriver
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

func systemReporter(threshold int) metrics.SystemReporter {
	return systemreporter.NewLogBased(time.Duration(threshold)*time.Second, commandrunner.New())
}

func newErrorHandler(logger lager.Logger, action string) exitErrorFunc {
	metricsEmitter := metrics.NewEmitter(systemReporter(0))

	return func(message string, exitCode int) *cli.ExitError {
		err := errors.New(message)
		defer metricsEmitter.TryIncrementRunCount(action, err)
		metricsEmitter.TryEmitError(logger, action, err, int32(exitCode))
		return cli.NewExitError(message, exitCode)
	}
}
