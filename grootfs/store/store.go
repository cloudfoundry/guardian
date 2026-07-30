package store // import "code.cloudfoundry.org/grootfs/store"
import "path/filepath"

const (
	ImageDirName     = "images"
	VolumesDirName   = "volumes"
	LocksDirName     = "locks"
	MetaDirName      = "meta"
	TempDirName      = "tmp"
	DefaultStorePath = "/var/lib/grootfs"
)

var StoreFolders []string = []string{
	ImageDirName,
	VolumesDirName,
	MetaDirName,
	LocksDirName,
	TempDirName,
	filepath.Join(MetaDirName, "dependencies"),
}
