package guardiancmd

import (
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/lager"
)

type SetupCommand struct {
	LogLevel LagerFlag
	Logger   lager.Logger

	Tag         string `hidden:"true" long:"tag" description:"Optional 2-character identifier used for namespacing global configuration."`
	RootlessUID *int   `hidden:"true" long:"rootless-uid" description:"UID that guardian will run as"`
	RootlessGID *int   `hidden:"true" long:"rootless-gid" description:"GID that guardian will run as"`
}

func (cmd *SetupCommand) Execute(args []string) error {
	cmd.Logger, _ = cmd.LogLevel.Logger("guardian-setup")

	chowner := &cgroups.OSChowner{UID: cmd.RootlessUID, GID: cmd.RootlessGID}
	cgroupStarter := wireCgroupsStarter(cmd.Logger, cmd.Tag, chowner)
	return cgroupStarter.Start()
}
