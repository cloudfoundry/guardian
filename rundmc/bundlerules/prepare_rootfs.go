package bundlerules

import (
	"os"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type mkdirChowner interface {
	MkdirAs(rootfsPath string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) error
}

type PrepareRootFS struct {
	ContainerRootUID int
	ContainerRootGID int
	MkdirChown       mkdirChowner
}

func (p PrepareRootFS) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec, _ string) (goci.Bndl, error) {
	var uid, gid int
	if !spec.Privileged {
		uid = p.ContainerRootUID
		gid = p.ContainerRootGID
	}

	p.MkdirChown.MkdirAs(
		spec.BaseConfig.Root.Path, uid, gid, 0755, true,
		"dev", "proc", "sys",
	)

	p.MkdirChown.MkdirAs(
		spec.BaseConfig.Root.Path, uid, gid, 0777, false,
		"tmp",
	)

	return bndl, nil
}
