package store // import "code.cloudfoundry.org/grootfs/store"

const (
	ImageDirName     = "images"
	VolumesDirName   = "volumes"
	CacheDirName     = "cache"
	LocksDirName     = "locks"
	MetaDirName      = "meta"
	TempDirName      = "tmp"
	DefaultStorePath = "/var/lib/grootfs"
	WhiteoutDevice   = "whiteout_dev"
)
