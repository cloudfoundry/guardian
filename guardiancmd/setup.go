package guardiancmd

import (
	"code.cloudfoundry.org/lager"
)

type SetupCommand struct {
	LogLevel LagerFlag
	Logger   lager.Logger

	Tag                 string `hidden:"true" long:"tag" description:"Optional 2-character identifier used for namespacing global configuration."`
	RootlessUID         *int   `hidden:"true" long:"rootless-uid" description:"UID that guardian will run as"`
	RootlessGID         *int   `hidden:"true" long:"rootless-gid" description:"GID that guardian will run as"`
	EnableCPUThrottling bool   `hidden:"true" long:"enable-cpu-throttling" description:"Throttle CPU of containers using more than their CPU entitlement"`
}

func (cmd *SetupCommand) Execute(args []string) error {
	cmd.Logger, _ = cmd.LogLevel.Logger("guardian-setup")
	cgroupStarter := cmd.WireCgroupsStarter(cmd.Logger)
	return cgroupStarter.Start()
}
