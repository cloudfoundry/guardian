package overlayxfs

const (
	UpperDir       = "diff"
	IDDir          = "projectids"
	WorkDir        = "workdir"
	RootfsDir      = "rootfs"
	imageInfoName  = "image_info"
	WhiteoutDevice = "whiteout_dev"
	LinksDirName   = "l"
)

func NewDriver(storePath, tardisBinPath string, externalLogSize int64) *Driver {
	return &Driver{
		storePath:       storePath,
		tardisBinPath:   tardisBinPath,
		externalLogSize: externalLogSize,
	}
}

type Driver struct {
	storePath       string
	tardisBinPath   string
	externalLogSize int64
}
