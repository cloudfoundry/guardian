package imageplugin

import (
	"os/exec"

	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/lager"
)

type NotImplementedCommandCreator struct {
	Err error
}

func (cc *NotImplementedCommandCreator) CreateCommand(log lager.Logger, handle string, spec rootfs_provider.Spec) (*exec.Cmd, error) {
	return nil, cc.Err
}

func (cc *NotImplementedCommandCreator) DestroyCommand(log lager.Logger, handle string) *exec.Cmd {
	return nil
}

func (cc *NotImplementedCommandCreator) MetricsCommand(log lager.Logger, handle string) *exec.Cmd {
	return nil
}
