package locksmith // import "code.cloudfoundry.org/grootfs/store/locksmith"
import "code.cloudfoundry.org/grootfs/groot"

const MetricsLockingTime = "LockingTime"

type FileSystem struct {
	storePath      string
	metricsEmitter groot.MetricsEmitter
}

func NewFileSystem(storePath string, metricsEmitter groot.MetricsEmitter) *FileSystem {
	return &FileSystem{
		storePath:      storePath,
		metricsEmitter: metricsEmitter,
	}
}
