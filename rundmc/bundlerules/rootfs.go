package bundlerules

import (
	"os"
	"os/exec"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
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

func (r RootFS) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec) goci.Bndl {
	var uid, gid int
	if !spec.Privileged {
		uid = r.ContainerRootUID
		gid = r.ContainerRootGID
	}

	r.MkdirChown.MkdirAs(
		spec.RootFSPath, uid, gid, 0755, true,
		".pivot_root",
		"dev", "proc", "sys",
	)

	r.MkdirChown.MkdirAs(
		spec.RootFSPath, uid, gid, 0777, false,
		"tmp",
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
