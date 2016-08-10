package fetcher

import (
	"fmt"
	"net/url"

	"github.com/containers/image/docker"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/lager"
)

type Fetcher struct {
}

func NewFetcher() *Fetcher {
	return &Fetcher{}
}

func (f *Fetcher) LayersDigest(logger lager.Logger, imageURL *url.URL) ([]string, error) {
	ref, err := docker.ParseReference("/" + imageURL.Path)
	if err != nil {
		return nil, fmt.Errorf("parsing url failed: %s", err)
	}

	img, err := ref.NewImage("", true)
	if err != nil {
		return nil, fmt.Errorf("creating an image: %s", err)
	}

	inspectInfo, err := img.Inspect()
	if err != nil {
		return nil, fmt.Errorf("inspecting image: %s", err)
	}

	return inspectInfo.Layers, nil
}

func (f *Fetcher) Streamer(logger lager.Logger, imageURL *url.URL) (cloner.Streamer, error) {
	ref, err := docker.ParseReference("/" + imageURL.Path)
	if err != nil {
		return nil, fmt.Errorf("parsing url failed: %s", err)
	}

	imgSrc, err := ref.NewImageSource("", true)
	if err != nil {
		return nil, fmt.Errorf("creating image source: %s", err)
	}

	return NewRemoteStreamer(imgSrc), nil
}
