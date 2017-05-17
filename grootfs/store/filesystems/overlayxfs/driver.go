package overlayxfs

const (
	UpperDir       = "diff"
	IDDir          = "projectids"
	WorkDir        = "workdir"
	RootfsDir      = "rootfs"
	imageInfoName  = "image_info"
	XfsType        = int64(0x58465342)
	WhiteoutDevice = "whiteout_dev"
	LinksDirName   = "l"
)

func NewDriver(storePath, tardisBinPath string) *Driver {
	return &Driver{
		storePath:     storePath,
		tardisBinPath: tardisBinPath,
	}
}

type Driver struct {
	storePath     string
	tardisBinPath string
}
