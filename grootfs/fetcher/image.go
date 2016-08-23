package fetcher

import (
	"encoding/json"
	"fmt"
	"io"

	"code.cloudfoundry.org/lager"
	"github.com/containers/image/types"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type StreamBlob func(logger lager.Logger) (io.ReadCloser, error)

//go:generate counterfeiter . CacheDriver
type CacheDriver interface {
	Blob(logger lager.Logger, id string, streamBlob StreamBlob) (io.ReadCloser, error)
}

type ContainersImage struct {
	ref            types.ImageReference
	cacheDriver    CacheDriver
	cachedManifest *specsv1.Manifest
}

func NewContainersImage(ref types.ImageReference, cacheDriver CacheDriver) *ContainersImage {
	return &ContainersImage{
		ref:         ref,
		cacheDriver: cacheDriver,
	}
}

func (i *ContainersImage) Manifest(logger lager.Logger) (specsv1.Manifest, error) {
	logger = logger.Session("fetching-image-manifest")
	logger.Info("start")
	defer logger.Info("end")

	if i.cachedManifest != nil {
		return *i.cachedManifest, nil
	}

	img, err := i.ref.NewImage("", false)
	if err != nil {
		return specsv1.Manifest{}, fmt.Errorf("creating image: %s", err)
	}

	contents, _, err := img.Manifest()
	if err != nil {
		return specsv1.Manifest{}, fmt.Errorf("fetching manifest: %s", err)
	}

	var manifest specsv1.Manifest
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return specsv1.Manifest{}, fmt.Errorf("parsing manifest: %s", err)
	}
	i.cachedManifest = &manifest

	return manifest, nil
}

func (i *ContainersImage) Config(logger lager.Logger) (specsv1.Image, error) {
	logger = logger.Session("fetching-image-config")
	logger.Info("start")
	defer logger.Info("end")

	manifest, err := i.Manifest(logger)
	if err != nil {
		return specsv1.Image{}, fmt.Errorf("fetching manifest: %s", err)
	}

	imgSrc, err := i.ref.NewImageSource("", false)
	if err != nil {
		return specsv1.Image{}, fmt.Errorf("creating image source: %s", err)
	}

	stream, err := i.cacheDriver.Blob(
		logger, manifest.Config.Digest,
		func(logger lager.Logger) (io.ReadCloser, error) {
			stream, _, err := imgSrc.GetBlob(manifest.Config.Digest)
			if err != nil {
				return nil, fmt.Errorf("fetching config blob: %s", err)
			}

			return stream, nil
		},
	)
	if err != nil {
		return specsv1.Image{}, err
	}

	var config specsv1.Image
	if err := json.NewDecoder(stream).Decode(&config); err != nil {
		return specsv1.Image{}, fmt.Errorf("parsing image config: %s", err)
	}

	return config, nil
}
