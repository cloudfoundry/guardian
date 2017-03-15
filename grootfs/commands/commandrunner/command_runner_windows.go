package commandrunner

import (
	cfcommandrunner "code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/windows_command_runner"
)

func New() cfcommandrunner.CommandRunner {
	return windows_command_runner.New(false)
}
