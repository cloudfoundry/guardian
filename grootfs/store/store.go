package store // import "code.cloudfoundry.org/grootfs/store"
import "code.cloudfoundry.org/lager"

const (
	IMAGES_DIR_NAME    = "images"
	VOLUMES_DIR_NAME   = "volumes"
	CACHE_DIR_NAME     = "cache"
	LOCKS_DIR_NAME     = "locks"
	META_DIR_NAME      = "meta"
	TEMP_DIR_NAME      = "tmp"
	DEFAULT_STORE_PATH = "/var/lib/grootfs"
)

//go:generate counterfeiter . VolumeDriver

type VolumeDriver interface {
	VolumePath(logger lager.Logger, id string) (string, error)
	CreateVolume(logger lager.Logger, parentID, id string) (string, error)
	DestroyVolume(logger lager.Logger, id string) error
	Volumes(logger lager.Logger) ([]string, error)
}
