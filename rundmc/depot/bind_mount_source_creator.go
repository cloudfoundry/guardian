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

//counterfeiter:generate . Chowner
type Chowner interface {
	Chown(path string, uid, gid int) error
}

// Create will create and chown a file in the container's depot to be
// bind-mounted into the container. The name of this file will be the basename
// of the mount destination. A consequence of this is that we currently can't
// create mountpoints with the same basename, e.g. /var/foo and /etc/foo. This
// restriction doesn't affect user-provided bind mounts.
func (b DepotBindMountSourceCreator) Create(containerDir string, chown bool) ([]garden.BindMount, error) {
	var bindMounts []garden.BindMount
	for _, mountPoint := range b.BindMountPoints {
		sourceFilename := filepath.Base(mountPoint)
		srcPath := filepath.Join(containerDir, sourceFilename)
		if err := b.createFile(srcPath, b.ContainerRootHostUID, b.ContainerRootHostGID, chown); err != nil {
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

func (b DepotBindMountSourceCreator) createFile(path string, containerRootHostUID, containerRootHostGID int, chown bool) error {
	if err := touchFile(path); err != nil {
		return err
	}

	if !chown {
		return nil
	}

	if err := b.Chowner.Chown(path, containerRootHostUID, containerRootHostGID); err != nil {
		wrappedErr := fmt.Errorf("error chowning %s: %s", filepath.Base(path), err)
		return wrappedErr
	}

	return nil
}

func touchFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}

type OSChowner struct{}

func (*OSChowner) Chown(path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}
