package bundlerules

import (
	"os"
	"path"
	"path/filepath"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
)

//go:generate counterfeiter . MkdirChowner

type RootFS struct {
	ContainerRootUID int
	ContainerRootGID int

	MkdirChowner MkdirChowner
}

func (r RootFS) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	os.RemoveAll(path.Join(spec.RootFSPath, "dev"))
	r.mkdirAsContainerRoot(filepath.Join(spec.RootFSPath, ".pivot_root"), 0700)
	r.mkdirAsContainerRoot(filepath.Join(spec.RootFSPath, "dev"), 0755)
	r.mkdirAsContainerRoot(filepath.Join(spec.RootFSPath, "proc"), 0755)
	r.mkdirAsContainerRoot(filepath.Join(spec.RootFSPath, "sys"), 0755)
	return bndl.WithRootFS(spec.RootFSPath)
}

func (r RootFS) mkdirAsContainerRoot(path string, perms os.FileMode) {
	r.MkdirChowner.MkdirChown(path, perms, r.ContainerRootUID, r.ContainerRootGID)
}
