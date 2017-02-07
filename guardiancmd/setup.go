package guardiancmd

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
)

type SetupCommand struct {
	Logger LagerFlag
	Tag    string `long:"tag" description:"Optional 2-character identifier used for namespacing global configuration."`
}

func (c *SetupCommand) Execute(args []string) error {
	logger, _ := c.Logger.Logger("guardian")
	cgroupStarter := wireCgroupStarter(logger, c.Tag)

	bulkStarter := BulkStarter{
		Starters: []gardener.Starter{
			cgroupStarter,
		},
	}

	return bulkStarter.StartAll()
}

type BulkStarter struct {
	Starters []gardener.Starter
}

func (b *BulkStarter) StartAll() error {
	for _, s := range b.Starters {
		if err := s.Start(); err != nil {
			return err
		}
	}
	return nil
}

func wireCgroupStarter(logger lager.Logger, tag string) gardener.Starter {
	var cgroupsMountpoint string
	if tag != "" {
		cgroupsMountpoint = filepath.Join(os.TempDir(), fmt.Sprintf("cgroups-%s", tag))
	} else {
		cgroupsMountpoint = "/sys/fs/cgroup"
	}

	return rundmc.NewStarter(logger, mustOpen("/proc/cgroups"), mustOpen("/proc/self/cgroup"), cgroupsMountpoint, linux_command_runner.New())
}
