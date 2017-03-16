package overlayxfs

const (
	UpperDir      = "diff"
	WorkDir       = "workdir"
	RootfsDir     = "rootfs"
	imageInfoName = "image_info"
	XfsType       = 0x58465342
)

func NewDriver(storePath string) *Driver {
	return &Driver{
		storePath: storePath,
	}
}

type Driver struct {
	storePath string
}
