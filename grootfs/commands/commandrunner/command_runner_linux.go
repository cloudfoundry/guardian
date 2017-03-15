package commandrunner

import (
	cfcommandrunner "code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/linux_command_runner"
)

func New() cfcommandrunner.CommandRunner {
	return linux_command_runner.New()
}
