package guardiancmd

import (
	"code.cloudfoundry.org/lager/v3"
)

type SetupCommand struct {
	LogLevel LagerFlag
	Logger   lager.Logger

	Tag                 string `hidden:"true" long:"tag" description:"Optional 2-character identifier used for namespacing global configuration."`
	EnableCPUThrottling bool   `hidden:"true" long:"enable-cpu-throttling" description:"Throttle CPU of containers using more than their CPU entitlement"`
}

func (cmd *SetupCommand) Execute(args []string) error {
	cmd.Logger, _ = cmd.LogLevel.Logger("guardian-setup")
	cgroupStarter := cmd.WireCgroupsStarter(cmd.Logger)
	return cgroupStarter.Start()
}
