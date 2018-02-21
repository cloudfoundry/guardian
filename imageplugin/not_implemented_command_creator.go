package imageplugin

import (
	"os/exec"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager"
)

type NotImplementedCommandCreator struct {
	Err error
}

func (cc *NotImplementedCommandCreator) CreateCommand(log lager.Logger, handle string, spec gardener.RootfsSpec) (*exec.Cmd, error) {
	return nil, cc.Err
}

func (cc *NotImplementedCommandCreator) DestroyCommand(log lager.Logger, handle string) *exec.Cmd {
	return nil
}

func (cc *NotImplementedCommandCreator) MetricsCommand(log lager.Logger, handle string) *exec.Cmd {
	return nil
}
