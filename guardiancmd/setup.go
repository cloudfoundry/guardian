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

	cgroupStarter := wireRunDMCStarter(logger, c.Tag)

	starters := []gardener.Starter{cgroupStarter}

	for _, s := range starters {
		if err := s.Start(); err != nil {
			panic(err)
		}
	}

	return nil
}

func wireRunDMCStarter(logger lager.Logger, tag string) gardener.Starter {
	var cgroupsMountpoint string
	if tag != "" {
		cgroupsMountpoint = filepath.Join(os.TempDir(), fmt.Sprintf("cgroups-%s", tag))
	} else {
		cgroupsMountpoint = "/sys/fs/cgroup"
	}

	return rundmc.NewStarter(logger, mustOpen("/proc/cgroups"), mustOpen("/proc/self/cgroup"), cgroupsMountpoint, linux_command_runner.New())
}
