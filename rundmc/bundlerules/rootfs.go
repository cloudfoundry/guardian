package bundlerules

import (
	"os"
	"os/exec"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type MkdirChowner interface {
	MkdirAs(rootfsPath string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) error
}

type RootFS struct {
	ContainerRootUID int
	ContainerRootGID int

	MkdirChown MkdirChowner
}

func (r RootFS) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec, _ string) (goci.Bndl, error) {
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

	return bndl.WithRootFS(spec.RootFSPath), nil
}

type ChrootMkdir struct {
	Command       func(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) *exec.Cmd
	CommandRunner commandrunner.CommandRunner
}

func (m ChrootMkdir) MkdirAs(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) error {
	return m.CommandRunner.Run(m.Command(
		rootFSPathFile,
		uid,
		gid,
		mode,
		recreate,
		paths...,
	))
}
