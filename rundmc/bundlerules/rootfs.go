package bundlerules

import (
	"os"
	"path/filepath"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
)

//go:generate counterfeiter . MkdirChowner
//go:generate counterfeiter . DirRemover

type RootFS struct {
	ContainerRootUID int
	ContainerRootGID int

	MkdirChowner MkdirChowner
	DirRemover   DirRemover
}

func (r RootFS) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	r.MkdirChowner.MkdirChown(filepath.Join(spec.RootFSPath, ".pivot_root"), 0700, r.ContainerRootUID, r.ContainerRootGID)
	r.DirRemover.Remove(filepath.Join(spec.RootFSPath, "dev", "shm"))
	return bndl.WithRootFS(spec.RootFSPath)
}

type MkdirChowner interface {
	MkdirChown(path string, perms os.FileMode, uid, gid int) error
}

type MkdirChownFunc func(path string, perms os.FileMode, uid, gid int) error

func (fn MkdirChownFunc) MkdirChown(path string, perms os.FileMode, uid, gid int) error {
	return fn(path, perms, uid, gid)
}

func MkdirChown(path string, perms os.FileMode, uid, gid int) error {
	if err := os.MkdirAll(path, perms); err != nil {
		return err
	}

	return os.Chown(path, uid, gid)
}

type DirRemover interface {
	Remove(name string) error
}

type OsDirRemover func(name string) error

func (fn OsDirRemover) Remove(name string) error {
	return fn(name)
}
