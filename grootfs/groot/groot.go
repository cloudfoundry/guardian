package groot // import "code.cloudfoundry.org/grootfs/groot"

import (
	"io"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	GlobalLockKey                      = "global-groot-lock"
	MetricImageCreationTime            = "ImageCreationTime"
	MetricImageDeletionTime            = "ImageDeletionTime"
	MetricImageStatsTime               = "ImageStatsTime"
	MetricImageCleanTime               = "ImageCleanTime"
	MetricDiskCachePercentage          = "DiskCachePercentage"
	MetricDiskCommittedPercentage      = "DiskCommittedPercentage"
	MetricDiskPurgeableCachePercentage = "DiskPurgeableCachePercentage"
)

//go:generate counterfeiter . ImageCloner
//go:generate counterfeiter . BaseImagePuller
//go:generate counterfeiter . Locksmith
//go:generate counterfeiter . DependencyManager
//go:generate counterfeiter . GarbageCollector
//go:generate counterfeiter . StoreMeasurer
//go:generate counterfeiter . RootFSConfigurer
//go:generate counterfeiter . MetricsEmitter
//go:generate counterfeiter . SandboxReexecer

type ImageInfo struct {
	Rootfs string        `json:"rootfs"`
	Image  specsv1.Image `json:"image,omitempty"`
	Mounts []MountInfo   `json:"mounts,omitempty"`
	Path   string        `json:"-"`
}

type MountInfo struct {
	Destination string   `json:"destination"`
	Type        string   `json:"type"`
	Source      string   `json:"source"`
	Options     []string `json:"options"`
}

type IDMappings struct {
	UIDMappings []IDMappingSpec
	GIDMappings []IDMappingSpec
}

type IDMappingSpec struct {
	HostID      int
	NamespaceID int
	Size        int
}

type BaseImageSpec struct {
	DiskLimit                 int64
	ExcludeBaseImageFromQuota bool
	UIDMappings               []IDMappingSpec
	GIDMappings               []IDMappingSpec
	OwnerUID                  int
	OwnerGID                  int
}

type LayerInfo struct {
	BlobID        string
	ChainID       string
	DiffID        string
	ParentChainID string
	Size          int64
	BaseDirectory string
	URLs          []string
	MediaType     string
}

type BaseImageInfo struct {
	LayerInfos []LayerInfo
	Config     specsv1.Image
}

type BaseImagePuller interface {
	FetchBaseImageInfo(logger lager.Logger) (BaseImageInfo, error)
	Pull(logger lager.Logger, imageInfo BaseImageInfo, spec BaseImageSpec) error
}

type ImageSpec struct {
	ID                        string
	Mount                     bool
	DiskLimit                 int64
	ExcludeBaseImageFromQuota bool
	BaseVolumeIDs             []string
	BaseImage                 specsv1.Image
	OwnerUID                  int
	OwnerGID                  int
}

type ImageCloner interface {
	Exists(id string) (bool, error)
	Create(logger lager.Logger, spec ImageSpec) (ImageInfo, error)
	Destroy(logger lager.Logger, id string) error
	Stats(logger lager.Logger, id string) (VolumeStats, error)
}

type RootFSConfigurer interface {
	Configure(rootFSPath string, baseImage *specsv1.Image) error
}

type DependencyManager interface {
	Register(id string, chainIDs []string) error
	Deregister(id string) error
}

type GarbageCollector interface {
	UnusedVolumes(logger lager.Logger) ([]string, error)
	MarkUnused(logger lager.Logger, unusedVolumes []string) error
	Collect(logger lager.Logger) error
}

type StoreMeasurer interface {
	CommittedQuota(logger lager.Logger) (int64, error)
	TotalVolumesSize(logger lager.Logger) (int64, error)
}

type Locksmith interface {
	Lock(key string) (*os.File, error)
	Unlock(lockFile *os.File) error
}

type MetricsEmitter interface {
	TryEmitUsage(logger lager.Logger, name string, usage int64, units string)
	TryEmitDurationFrom(logger lager.Logger, name string, from time.Time)
}

type SandboxReexecer interface {
	Reexec(commandName string, spec ReexecSpec) ([]byte, error)
}

type ReexecSpec struct {
	Stdin       io.Reader
	ChrootDir   string
	ExtraFiles  []string
	Args        []string
	CloneUserns bool
}

type DiskUsage struct {
	TotalBytesUsed     int64 `json:"total_bytes_used"`
	ExclusiveBytesUsed int64 `json:"exclusive_bytes_used"`
}

type VolumeStats struct {
	DiskUsage DiskUsage `json:"disk_usage"`
}
