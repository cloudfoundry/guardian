package bundlerules

import (
	"os"
	"os/exec"

	"code.cloudfoundry.org/commandrunner"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type RootFS struct {
}

func (r RootFS) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec) (goci.Bndl, error) {
	return bndl.WithRootFS(spec.BaseConfig.Root.Path), nil
}

type MkdirChowner struct {
	Command       func(spec specs.Spec, uid, gid int, mode os.FileMode, recreate bool, paths ...string) (*exec.Cmd, error)
	CommandRunner commandrunner.CommandRunner
}

func (m MkdirChowner) MkdirAs(spec specs.Spec, uid, gid int, mode os.FileMode, recreate bool, paths ...string) error {
	cmd, err := m.Command(
		spec,
		uid,
		gid,
		mode,
		recreate,
		paths...,
	)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return m.CommandRunner.Run(cmd)
}
