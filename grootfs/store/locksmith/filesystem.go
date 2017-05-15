package locksmith // import "code.cloudfoundry.org/grootfs/store/locksmith"

import "code.cloudfoundry.org/grootfs/groot"

const ExclusiveMetricsLockingTime = "ExclusiveLockingTime"
const SharedMetricsLockingTime = "SharedLockingTime"

type FileSystem struct {
	storePath      string
	metricsEmitter groot.MetricsEmitter
	lockType       int
	metricName     string
}
