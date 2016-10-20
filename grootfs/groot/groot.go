package groot // import "code.cloudfoundry.org/grootfs/groot"

import (
	"fmt"
	"net/url"
	"os"

	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	errorspkg "github.com/pkg/errors"
)

const GLOBAL_LOCK_KEY = "global-groot-lock"

//go:generate counterfeiter . Bundler
//go:generate counterfeiter . ImagePuller
//go:generate counterfeiter . Locksmith
//go:generate counterfeiter . DependencyManager
//go:generate counterfeiter . GarbageCollector

type IDMappingSpec struct {
	HostID      int
	NamespaceID int
	Size        int
}

type ImageSpec struct {
	DiskLimit             int64
	ExcludeImageFromQuota bool
	ImageSrc              *url.URL
	UIDMappings           []IDMappingSpec
	GIDMappings           []IDMappingSpec
}

type Image struct {
	VolumePath string
	Image      specsv1.Image
	ChainIDs   []string
}

type ImagePuller interface {
	Pull(logger lager.Logger, spec ImageSpec) (Image, error)
}

type BundleSpec struct {
	ID                    string
	DiskLimit             int64
	ExcludeImageFromQuota bool
	VolumePath            string
	Image                 specsv1.Image
}

type Bundle struct {
	Path       string
	RootFSPath string
}

type Bundler interface {
	Exists(id string) (bool, error)
	BundleIDs(logger lager.Logger) ([]string, error)
	Create(logger lager.Logger, spec BundleSpec) (Bundle, error)
	Destroy(logger lager.Logger, id string) error
	Metrics(logger lager.Logger, id string) (VolumeMetrics, error)
}

type DependencyManager interface {
	Dependencies(id string) ([]string, error)
	Register(id string, chainIDs []string) error
	Deregister(id string) error
}

type GarbageCollector interface {
	Collect(lager.Logger) error
}

type Locksmith interface {
	Lock(key string) (*os.File, error)
	Unlock(lockFile *os.File) error
}

type Groot struct {
	bundler           Bundler
	garbageCollector  GarbageCollector
	dependencyManager DependencyManager
	imagePuller       ImagePuller
	locksmith         Locksmith
}

type DiskUsage struct {
	TotalBytesUsed     int64 `json:"total_bytes_used"`
	ExclusiveBytesUsed int64 `json:"exclusive_bytes_used"`
}

type VolumeMetrics struct {
	DiskUsage DiskUsage `json:"disk_usage"`
}

func IamGroot(bundler Bundler, imagePuller ImagePuller, locksmith Locksmith, dependencyManager DependencyManager, gc GarbageCollector) *Groot {
	return &Groot{
		bundler:           bundler,
		imagePuller:       imagePuller,
		locksmith:         locksmith,
		dependencyManager: dependencyManager,
		garbageCollector:  gc,
	}
}

type CreateSpec struct {
	ID                    string
	Image                 string
	DiskLimit             int64
	ExcludeImageFromQuota bool
	UIDMappings           []IDMappingSpec
	GIDMappings           []IDMappingSpec
}

func (g *Groot) Create(logger lager.Logger, spec CreateSpec) (Bundle, error) {
	logger = logger.Session("groot-creating", lager.Data{"bundleID": spec.ID, "spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	parsedURL, err := url.Parse(spec.Image)
	if err != nil {
		return Bundle{}, fmt.Errorf("parsing image url: %s", err)
	}

	ok, err := g.bundler.Exists(spec.ID)
	if err != nil {
		return Bundle{}, fmt.Errorf("checking id exists: %s", err)
	}
	if ok {
		return Bundle{}, fmt.Errorf("bundle for id `%s` already exists", spec.ID)
	}

	imageSpec := ImageSpec{
		ImageSrc:              parsedURL,
		DiskLimit:             spec.DiskLimit,
		ExcludeImageFromQuota: spec.ExcludeImageFromQuota,
		UIDMappings:           spec.UIDMappings,
		GIDMappings:           spec.GIDMappings,
	}

	lockFile, err := g.locksmith.Lock(GLOBAL_LOCK_KEY)
	if err != nil {
		return Bundle{}, err
	}

	image, err := g.imagePuller.Pull(logger, imageSpec)
	if err != nil {
		if err := g.locksmith.Unlock(lockFile); err != nil {
			logger.Error("failed-to-unlock", err)
		}

		return Bundle{}, errorspkg.Wrap(err, "pulling the image")
	}

	if err := g.locksmith.Unlock(lockFile); err != nil {
		logger.Error("failed-to-unlock", err)
	}

	bundleSpec := BundleSpec{
		ID:                    spec.ID,
		DiskLimit:             spec.DiskLimit,
		ExcludeImageFromQuota: spec.ExcludeImageFromQuota,
		VolumePath:            image.VolumePath,
		Image:                 image.Image,
	}
	bundle, err := g.bundler.Create(logger, bundleSpec)
	if err != nil {
		return Bundle{}, fmt.Errorf("making bundle: %s", err)
	}

	if err := g.dependencyManager.Register(spec.ID, image.ChainIDs); err != nil {
		if destroyErr := g.bundler.Destroy(logger, spec.ID); destroyErr != nil {
			logger.Error("failed-to-destroy-bundle", destroyErr)
		}

		return Bundle{}, err
	}

	return bundle, nil
}

func (g *Groot) Delete(logger lager.Logger, id string) error {
	logger = logger.Session("groot-deleting", lager.Data{"bundleID": id})
	logger.Info("start")
	defer logger.Info("end")

	err := g.bundler.Destroy(logger, id)
	if derErr := g.dependencyManager.Deregister(id); derErr != nil {
		logger.Error("failed-to-deregister-dependencies", derErr)
	}

	return err
}

func (g *Groot) Metrics(logger lager.Logger, id string) (VolumeMetrics, error) {
	logger = logger.Session("groot-metrics", lager.Data{"bundleID": id})
	logger.Info("start")
	defer logger.Info("end")

	metrics, err := g.bundler.Metrics(logger, id)
	if err != nil {
		return VolumeMetrics{}, fmt.Errorf("fetching metrics for `%s`: %s", id, err)
	}

	return metrics, nil
}

func (g *Groot) Clean(logger lager.Logger) error {
	logger = logger.Session("groot-cleaning")
	logger.Info("start")
	defer logger.Info("end")

	lockFile, err := g.locksmith.Lock(GLOBAL_LOCK_KEY)
	if err != nil {
		return err
	}
	defer func() {
		if err := g.locksmith.Unlock(lockFile); err != nil {
			logger.Error("failed-to-unlock", err)
		}
	}()

	return g.garbageCollector.Collect(logger)
}
