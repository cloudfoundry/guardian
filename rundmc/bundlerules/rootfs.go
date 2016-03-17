package bundlerules

import (
	"os"
	"os/exec"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry/gunk/command_runner"
)

type MkdirChowner interface {
	MkdirAs(rootfsPath string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) error
}

type RootFS struct {
	ContainerRootUID int
	ContainerRootGID int

	MkdirChown MkdirChowner
}

func (r RootFS) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	r.MkdirChown.MkdirAs(
		spec.RootFSPath, r.ContainerRootUID, r.ContainerRootGID, 0755, true,
		".pivot_root",
		"dev", "proc", "sys",
	)

	return bndl.WithRootFS(spec.RootFSPath)
}

type ChrootMkdir struct {
	Command       func(rootfsPath string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) *exec.Cmd
	CommandRunner command_runner.CommandRunner
}

func (m ChrootMkdir) MkdirAs(rootfsPath string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) error {
	return m.CommandRunner.Run(m.Command(
		rootfsPath,
		uid,
		gid,
		mode,
		recreate,
		paths...,
	))
}
