package fetcher

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/containers/image/types"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type ContainersImage struct {
	ref types.ImageReference
}

func NewContainersImage(ref types.ImageReference) *ContainersImage {
	return &ContainersImage{ref: ref}
}

func (i *ContainersImage) Manifest(logger lager.Logger) (specsv1.Manifest, error) {
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

	return manifest, nil
}

func (i *ContainersImage) Config(logger lager.Logger) (specsv1.Image, error) {
	manifest, err := i.Manifest(logger)
	if err != nil {
		return specsv1.Image{}, fmt.Errorf("fetching manifest: %s", err)
	}

	imgSrc, err := i.ref.NewImageSource("", false)
	if err != nil {
		return specsv1.Image{}, fmt.Errorf("creating image source: %s", err)
	}

	stream, _, err := imgSrc.GetBlob(manifest.Config.Digest)
	if err != nil {
		return specsv1.Image{}, fmt.Errorf("fetching config blob: %s", err)
	}

	var config specsv1.Image
	if err := json.NewDecoder(stream).Decode(&config); err != nil {
		return specsv1.Image{}, fmt.Errorf("parsing image config: %s", err)
	}

	return config, nil
}
