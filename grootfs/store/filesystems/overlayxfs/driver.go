package overlayxfs

import "code.cloudfoundry.org/grootfs/store/filesystems"

const (
	BaseFileSystemName = "xfs"
	UpperDir           = "diff"
	WorkDir            = "workdir"
	RootfsDir          = "rootfs"
	imageInfoName      = "image_info"
)

func NewDriver(storePath string) (*Driver, error) {
	if err := filesystems.CheckFSPath(storePath, filesystems.XfsType, BaseFileSystemName); err != nil {
		return nil, err
	}
	return &Driver{
		storePath: storePath,
	}, nil
}

type Driver struct {
	storePath string
}
