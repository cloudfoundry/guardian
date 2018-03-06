package bundlerules

import (
	"os"
	"os/exec"

	"code.cloudfoundry.org/commandrunner"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type RootFS struct {
}

func (r RootFS) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec, _ string) (goci.Bndl, error) {
	return bndl.WithRootFS(spec.BaseConfig.Root.Path), nil
}

type MkdirChowner struct {
	Command       func(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) *exec.Cmd
	CommandRunner commandrunner.CommandRunner
}

func (m MkdirChowner) MkdirAs(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) error {
	return m.CommandRunner.Run(m.Command(
		rootFSPathFile,
		uid,
		gid,
		mode,
		recreate,
		paths...,
	))
}
