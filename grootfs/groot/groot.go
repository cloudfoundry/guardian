package groot

import (
	"fmt"
	"net/url"
	"os"

	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const GLOBAL_LOCK_KEY = "global-groot-lock"

//go:generate counterfeiter . Bundler
//go:generate counterfeiter . Bundle
//go:generate counterfeiter . ImagePuller
//go:generate counterfeiter . Locksmith

type Bundle interface {
	Path() string
	RootFSPath() string
}

type BundleSpec struct {
	DiskLimit      int64
	ExclusiveLimit bool
	VolumePath     string
	Image          specsv1.Image
}

type Bundler interface {
	Exists(id string) (bool, error)
	Create(logger lager.Logger, id string, spec BundleSpec) (Bundle, error)
	Destroy(logger lager.Logger, id string) error
	Metrics(logger lager.Logger, id string, forceSync bool) (VolumeMetrics, error)
}

type Locksmith interface {
	Lock(key string) (*os.File, error)
	Unlock(lockFile *os.File) error
}

type IDMappingSpec struct {
	HostID      int
	NamespaceID int
	Size        int
}

type ImageSpec struct {
	ImageSrc    *url.URL
	UIDMappings []IDMappingSpec
	GIDMappings []IDMappingSpec
}

type ImagePuller interface {
	Pull(logger lager.Logger, spec ImageSpec) (BundleSpec, error)
}

type Groot struct {
	bundler     Bundler
	imagePuller ImagePuller
	locksmith   Locksmith
}

type DiskUsage struct {
	TotalBytesUsed     int64 `json:"total_bytes_used"`
	ExclusiveBytesUsed int64 `json:"exclusive_bytes_used"`
}
type VolumeMetrics struct {
	DiskUsage DiskUsage `json:"disk_usage"`
}

func IamGroot(bundler Bundler, imagePuller ImagePuller, locksmith Locksmith) *Groot {
	return &Groot{
		bundler:     bundler,
		imagePuller: imagePuller,
		locksmith:   locksmith,
	}
}

type CreateSpec struct {
	ID             string
	Image          string
	DiskLimit      int64
	ExclusiveLimit bool
	UIDMappings    []IDMappingSpec
	GIDMappings    []IDMappingSpec
}

func (g *Groot) Create(logger lager.Logger, spec CreateSpec) (Bundle, error) {
	logger = logger.Session("groot-creating", lager.Data{"bundleID": spec.ID, "spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	parsedURL, err := url.Parse(spec.Image)
	if err != nil {
		return nil, fmt.Errorf("parsing image url: %s", err)
	}

	ok, err := g.bundler.Exists(spec.ID)
	if err != nil {
		return nil, fmt.Errorf("checking id exists: %s", err)
	}
	if ok {
		return nil, fmt.Errorf("bundle for id `%s` already exists", spec.ID)
	}

	imageSpec := ImageSpec{
		ImageSrc:    parsedURL,
		UIDMappings: spec.UIDMappings,
		GIDMappings: spec.GIDMappings,
	}

	lockFile, err := g.locksmith.Lock(GLOBAL_LOCK_KEY)
	if err != nil {
		return nil, err
	}

	bundleSpec, err := g.imagePuller.Pull(logger, imageSpec)
	if err != nil {
		if err := g.locksmith.Unlock(lockFile); err != nil {
			logger.Error("failed-to-unlock", err)
		}

		return nil, fmt.Errorf("pulling the image: %s", err)
	}

	if err := g.locksmith.Unlock(lockFile); err != nil {
		logger.Error("failed-to-unlock", err)
	}

	bundleSpec.DiskLimit = spec.DiskLimit
	bundleSpec.ExclusiveLimit = spec.ExclusiveLimit
	bundle, err := g.bundler.Create(logger, spec.ID, bundleSpec)
	if err != nil {
		return nil, fmt.Errorf("making bundle: %s", err)
	}

	return bundle, nil
}

func (g *Groot) Delete(logger lager.Logger, id string) error {
	return g.bundler.Destroy(logger, id)
}

func (g *Groot) Metrics(logger lager.Logger, id string, forceSync bool) (VolumeMetrics, error) {
	logger = logger.Session("groot-metrics", lager.Data{"bundleID": id, "forceSync": forceSync})
	logger.Info("start")
	defer logger.Info("end")

	metrics, err := g.bundler.Metrics(logger, id, forceSync)
	if err != nil {
		return VolumeMetrics{}, fmt.Errorf("fetching metrics for `%s`: %s", id, err)
	}

	return metrics, nil
}
