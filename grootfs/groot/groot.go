package groot // import "code.cloudfoundry.org/grootfs/groot"

import (
	"net/url"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	GlobalLockKey           = "global-groot-lock"
	MetricImageCreationTime = "ImageCreationTime"
	MetricImageDeletionTime = "ImageDeletionTime"
	MetricImageStatsTime    = "ImageStatsTime"
	MetricImageCleanTime    = "ImageCleanTime"
)

//go:generate counterfeiter . ImageCloner
//go:generate counterfeiter . BaseImagePuller
//go:generate counterfeiter . Locksmith
//go:generate counterfeiter . DependencyManager
//go:generate counterfeiter . GarbageCollector
//go:generate counterfeiter . StoreMeasurer
//go:generate counterfeiter . RootFSConfigurer
//go:generate counterfeiter . MetricsEmitter

type IDMappingSpec struct {
	HostID      int
	NamespaceID int
	Size        int
}

type BaseImageSpec struct {
	DiskLimit                 int64
	ExcludeBaseImageFromQuota bool
	BaseImageSrc              *url.URL
	UIDMappings               []IDMappingSpec
	GIDMappings               []IDMappingSpec
}

type BaseImage struct {
	VolumePath string
	BaseImage  specsv1.Image
	ChainIDs   []string
}

type BaseImagePuller interface {
	Pull(logger lager.Logger, spec BaseImageSpec) (BaseImage, error)
}

type ImageSpec struct {
	ID                        string
	DiskLimit                 int64
	ExcludeBaseImageFromQuota bool
	VolumePath                string
	BaseImage                 specsv1.Image
}

type Image struct {
	Path       string
	RootFSPath string
}

type ImageCloner interface {
	Exists(id string) (bool, error)
	Create(logger lager.Logger, spec ImageSpec) (Image, error)
	Destroy(logger lager.Logger, id string) error
	Stats(logger lager.Logger, id string) (VolumeStats, error)
}

type RootFSConfigurer interface {
	Configure(rootFSPath string, baseImage specsv1.Image) error
}

type DependencyManager interface {
	Register(id string, chainIDs []string) error
	Deregister(id string) error
}

type GarbageCollector interface {
	Collect(logger lager.Logger, keepBaseImages []string) error
}

type StoreMeasurer interface {
	MeasureStore(logger lager.Logger) (uint64, error)
}

type Locksmith interface {
	Lock(key string) (*os.File, error)
	Unlock(lockFile *os.File) error
}

type MetricsEmitter interface {
	TryEmitDuration(logger lager.Logger, name string, duration time.Duration)
}

type DiskUsage struct {
	TotalBytesUsed     int64 `json:"total_bytes_used"`
	ExclusiveBytesUsed int64 `json:"exclusive_bytes_used"`
}

type VolumeStats struct {
	DiskUsage DiskUsage `json:"disk_usage"`
}
