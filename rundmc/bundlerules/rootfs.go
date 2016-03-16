package bundlerules

import (
	"os"
	"os/exec"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry/gunk/command_runner"
)

type MkdirChowner interface {
	MkdirChown(rootfsPath string, uid, gid int, mode os.FileMode, paths ...string)
}

type RootFS struct {
	ContainerRootUID int
	ContainerRootGID int

	MkdirChown MkdirChowner
}

func (r RootFS) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	r.MkdirChown.MkdirChown(
		spec.RootFSPath, r.ContainerRootUID, r.ContainerRootGID, 0700,
		".pivot_root",
	)
	r.MkdirChown.MkdirChown(
		spec.RootFSPath, r.ContainerRootUID, r.ContainerRootGID, 0755,
		"dev", "proc", "sys",
	)

	return bndl.WithRootFS(spec.RootFSPath)
}

type Mkdir struct {
	Command       func(rootfsPath string, uid, gid int, mode os.FileMode, paths ...string) *exec.Cmd
	CommandRunner command_runner.CommandRunner
}

func (m Mkdir) MkdirChown(rootfsPath string, uid, gid int, mode os.FileMode, paths ...string) {
	m.CommandRunner.Run(m.Command(
		rootfsPath,
		uid,
		gid,
		mode,
		paths...,
	))
}
