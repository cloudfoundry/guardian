package store // import "code.cloudfoundry.org/grootfs/store"
import "path/filepath"

const (
	ImageDirName     = "images"
	VolumesDirName   = "volumes"
	CacheDirName     = "cache"
	LocksDirName     = "locks"
	MetaDirName      = "meta"
	TempDirName      = "tmp"
	DefaultStorePath = "/var/lib/grootfs"
)

var StoreFolders []string = []string{
	ImageDirName,
	VolumesDirName,
	CacheDirName,
	MetaDirName,
	LocksDirName,
	TempDirName,
	filepath.Join(MetaDirName, "dependencies"),
}
