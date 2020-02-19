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

func (r RootFS) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec) (goci.Bndl, error) {
	return bndl.WithRootFS(spec.BaseConfig.Root.Path), nil
}

type MkdirChowner struct {
	Command       func(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) *exec.Cmd
	CommandRunner commandrunner.CommandRunner
}

func (m MkdirChowner) MkdirAs(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) error {
	stdout, err := os.Create("/var/vcap/data/garden/stdout")
	if err != nil {
		return err
	}
	defer stdout.Close()

	stderr, err := os.Create("/var/vcap/data/garden/stderr")
	if err != nil {
		return err
	}
	defer stderr.Close()

	cmd := m.Command(
		rootFSPathFile,
		uid,
		gid,
		mode,
		recreate,
		paths...,
	)

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return m.CommandRunner.Run(cmd)
}
