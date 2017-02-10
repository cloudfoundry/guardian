package guardiancmd

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/guardian/logging"
	locksmithpkg "code.cloudfoundry.org/guardian/pkg/locksmith"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
)

type SetupCommand struct {
	LogLevel LagerFlag
	Logger   lager.Logger

	Tag string `hidden:"true" long:"tag" description:"Optional 2-character identifier used for namespacing global configuration."`

	Network struct {
		IPTables           FileFlag   `long:"iptables-bin"  default:"/sbin/iptables" description:"path to the iptables binary"`
		AllowHostAccess    bool       `long:"allow-host-access" description:"Allow network access to the host machine."`
		DenyNetworks       []CIDRFlag `long:"deny-network"      description:"Network ranges to which traffic from containers will be denied. Can be specified multiple times."`
		ResetIPTablesRules bool       `long:"reset-iptables-rules" description:"Clears all garden-created iptables rules. Don’t use this unless you plan to destroy all containers."`
	}
}

func (cmd *SetupCommand) Execute(args []string) error {
	cmd.Logger, _ = cmd.LogLevel.Logger("guardian-setup")

	bulkStarter := gardener.NewBulkStarter([]gardener.Starter{
		cmd.wireCgroupStarter(),
		cmd.wireIPTablesStarter(),
	})

	return bulkStarter.StartAll()
}

func (cmd *SetupCommand) wireCgroupStarter() gardener.Starter {
	var cgroupsMountpoint string
	if cmd.Tag != "" {
		cgroupsMountpoint = filepath.Join(os.TempDir(), fmt.Sprintf("cgroups-%s", cmd.Tag))
	} else {
		cgroupsMountpoint = "/sys/fs/cgroup"
	}

	return rundmc.NewStarter(cmd.Logger, mustOpen("/proc/cgroups"), mustOpen("/proc/self/cgroup"), cgroupsMountpoint, linux_command_runner.New())
}

func (cmd *SetupCommand) wireIPTablesStarter() gardener.Starter {
	var denyNetworksList []string
	for _, network := range cmd.Network.DenyNetworks {
		denyNetworksList = append(denyNetworksList, network.String())
	}

	interfacePrefix := fmt.Sprintf("w%s", cmd.Tag)
	chainPrefix := fmt.Sprintf("w-%s-", cmd.Tag)
	iptRunner := &logging.Runner{CommandRunner: linux_command_runner.New(), Logger: cmd.Logger.Session("iptables-runner")}
	locksmith := &locksmithpkg.FileSystem{}
	ipTables := iptables.New(cmd.Network.IPTables.Path(), "iptables-restore-not-used", iptRunner, locksmith, chainPrefix)
	ipTablesStarter := iptables.NewStarter(ipTables, cmd.Network.AllowHostAccess, interfacePrefix, denyNetworksList, cmd.Network.ResetIPTablesRules)

	return ipTablesStarter
}
