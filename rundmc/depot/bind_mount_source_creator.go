package depot

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
)

type DepotBindMountSourceCreator struct {
	BindMountPoints      []string
	Chowner              Chowner
	ContainerRootHostUID int
	ContainerRootHostGID int
}

//go:generate counterfeiter . Chowner
type Chowner interface {
	Chown(path string, uid, gid int) error
}

func (r DepotBindMountSourceCreator) Create(containerDir string, chown bool) ([]garden.BindMount, error) {
	var bindMounts []garden.BindMount
	for _, mountPoint := range r.BindMountPoints {
		sourceFilename := filepath.Base(mountPoint)
		srcPath := filepath.Join(containerDir, sourceFilename)
		if err := r.createFile(srcPath, r.ContainerRootHostUID, r.ContainerRootHostGID, chown); err != nil {
			return nil, err
		}
		bindMounts = append(bindMounts, garden.BindMount{
			SrcPath: srcPath,
			DstPath: mountPoint,
			Mode:    garden.BindMountModeRW,
		})
	}

	return bindMounts, nil
}

func (r DepotBindMountSourceCreator) createFile(path string, containerRootHostUID, containerRootHostGID int, chown bool) error {
	if err := touchFile(path); err != nil {
		return err
	}

	if !chown {
		return nil
	}

	if err := r.Chowner.Chown(path, containerRootHostUID, containerRootHostGID); err != nil {
		wrappedErr := fmt.Errorf("error chowning %s: %s", filepath.Base(path), err)
		return wrappedErr
	}

	return nil
}

func touchFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}

type OSChowner struct{}

func (*OSChowner) Chown(path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}
