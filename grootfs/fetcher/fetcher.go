package fetcher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/containers/image/docker"
	"github.com/containers/image/types"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/lager"

	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

//go:generate counterfeiter . Image
type Image interface {
	Manifest(logger lager.Logger) (specsv1.Manifest, error)
	Config(logger lager.Logger) (specsv1.Image, error)
}

type ImageProvider func(ref types.ImageReference) Image

type Fetcher struct {
	cachePath     string
	imageProvider ImageProvider
}

func NewFetcher(cachePath string, imageProvider ImageProvider) *Fetcher {
	return &Fetcher{
		cachePath:     cachePath,
		imageProvider: imageProvider,
	}
}

func (f *Fetcher) LayersDigest(logger lager.Logger, imageURL *url.URL) ([]cloner.LayerDigest, error) {
	logger = logger.Session("layers-digest", lager.Data{"imageURL": imageURL})
	logger.Info("start")
	defer logger.Info("end")

	logger.Debug("parsing-reference")
	refString := "/"
	if imageURL.Host != "" {
		refString += "/" + imageURL.Host
	}
	refString += imageURL.Path

	ref, err := docker.ParseReference(refString)
	if err != nil {
		return nil, fmt.Errorf("parsing url failed: %s", err)
	}

	img := f.imageProvider(ref)

	logger.Debug("fetching-image-manifest")
	manifest, err := img.Manifest(logger)
	if err != nil {
		return nil, fmt.Errorf("getting image manifest: %s", err)
	}

	logger.Debug("fetching-image-config")
	config, err := img.Config(logger)
	if err != nil {
		return nil, fmt.Errorf("getting image config: %s", err)
	}

	return f.createLayersDigest(logger, manifest, config), nil
}

func (f *Fetcher) Streamer(logger lager.Logger, imageURL *url.URL) (cloner.Streamer, error) {
	logger = logger.Session("streaming", lager.Data{"imageURL": imageURL})
	logger.Info("start")
	defer logger.Info("end")

	logger.Debug("parsing-reference")
	ref, err := docker.ParseReference("/" + imageURL.Path)
	if err != nil {
		return nil, fmt.Errorf("parsing url failed: %s", err)
	}

	logger.Debug("parsing-image-source")
	imgSrc, err := ref.NewImageSource("", true)
	if err != nil {
		return nil, fmt.Errorf("creating image source: %s", err)
	}

	remoteStreamer := NewRemoteStreamer(imgSrc)
	return NewCachedStreamer(f.cachePath, remoteStreamer), nil
}

func (f *Fetcher) createLayersDigest(logger lager.Logger, manifest specsv1.Manifest, config specsv1.Image) []cloner.LayerDigest {
	layersDigest := []cloner.LayerDigest{}

	var parentChainID string
	for i, blobDesc := range manifest.Layers {
		if i == 0 {
			parentChainID = ""
		}

		diffID := config.RootFS.DiffIDs[i]
		chainID := f.chainID(diffID, parentChainID)
		layersDigest = append(layersDigest, cloner.LayerDigest{
			BlobID:        blobDesc.Digest,
			DiffID:        diffID,
			ChainID:       chainID,
			ParentChainID: parentChainID,
		})
		parentChainID = chainID
	}

	return layersDigest
}

func (f *Fetcher) chainID(diffID string, parentChainID string) string {
	diffID = strings.Split(diffID, ":")[1]
	chainID := diffID

	if parentChainID != "" {
		parentChainID = strings.Split(parentChainID, ":")[1]
		chainIDSha := sha256.Sum256([]byte(fmt.Sprintf("%s %s", parentChainID, diffID)))
		chainID = hex.EncodeToString(chainIDSha[:32])
	}

	return "sha256:" + chainID
}
