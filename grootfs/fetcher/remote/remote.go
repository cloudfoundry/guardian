package remote // import "code.cloudfoundry.org/grootfs/fetcher/remote"

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/grootfs/image_puller"
	"code.cloudfoundry.org/lager"

	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

//go:generate counterfeiter . Source
type Source interface {
	Manifest(logger lager.Logger, imageURL *url.URL) (Manifest, error)
	Config(logger lager.Logger, imageURL *url.URL, manifest Manifest) (specsv1.Image, error)
	Blob(logger lager.Logger, imageURL *url.URL, digest string) ([]byte, int64, error)
}

type RemoteFetcher struct {
	source      Source
	cacheDriver fetcher.CacheDriver
}

func NewRemoteFetcher(source Source, cacheDriver fetcher.CacheDriver) *RemoteFetcher {
	return &RemoteFetcher{
		source:      source,
		cacheDriver: cacheDriver,
	}
}

func (f *RemoteFetcher) ImageInfo(logger lager.Logger, imageURL *url.URL) (image_puller.ImageInfo, error) {
	logger = logger.Session("layers-digest", lager.Data{"imageURL": imageURL})
	logger.Info("start")
	defer logger.Info("end")

	logger.Debug("fetching-image-manifest")
	manifest, err := f.source.Manifest(logger, imageURL)
	if err != nil {
		return image_puller.ImageInfo{}, err
	}
	logger.Debug("image-manifest", lager.Data{"manifest": manifest})

	logger.Debug("fetching-image-config")
	stream, err := f.cacheDriver.StreamBlob(logger, manifest.ConfigCacheKey,
		func(logger lager.Logger) ([]byte, int64, error) {
			config, err := f.source.Config(logger, imageURL, manifest)
			if err != nil {
				return nil, 0, err
			}

			configJSON, err := json.Marshal(config)
			if err != nil {
				return nil, 0, fmt.Errorf("encoding config to JSON: %s", err)
			}

			return configJSON, 0, nil
		},
	)
	if err != nil {
		return image_puller.ImageInfo{}, err
	}

	var config specsv1.Image
	if err := json.NewDecoder(stream.Reader).Decode(&config); err != nil {
		stream.Cancel()
		return image_puller.ImageInfo{}, fmt.Errorf("decoding config from JSON: %s", err)
	}
	logger.Debug("image-config", lager.Data{"config": config})

	return image_puller.ImageInfo{
		LayersDigest: f.createLayersDigest(logger, manifest, config),
		Config:       config,
	}, nil
}

func (f *RemoteFetcher) StreamBlob(logger lager.Logger, imageURL *url.URL, source string) (image_puller.Stream, error) {
	logger = logger.Session("streaming", lager.Data{"imageURL": imageURL})
	logger.Info("start")
	defer logger.Info("end")

	stream, err := f.cacheDriver.StreamBlob(logger, source,
		func(logger lager.Logger) ([]byte, int64, error) {
			blobContents, size, err := f.source.Blob(logger, imageURL, source)
			if err != nil {
				return nil, 0, err
			}

			return blobContents, size, nil
		},
	)
	if err != nil {
		return image_puller.Stream{}, err
	}

	reader, err := gzip.NewReader(stream.Reader)
	if err != nil {
		stream.Cancel()
		return image_puller.Stream{}, err
	}
	return image_puller.Stream{
		Reader: reader,
		Size:   stream.Size,
		Cancel: stream.Cancel,
	}, nil
}

func (f *RemoteFetcher) createLayersDigest(logger lager.Logger,
	manifest Manifest, config specsv1.Image,
) []image_puller.LayerDigest {
	layersDigest := []image_puller.LayerDigest{}

	var parentChainID string
	for i, layer := range manifest.Layers {
		if i == 0 {
			parentChainID = ""
		}

		diffID := config.RootFS.DiffIDs[i]
		chainID := f.chainID(diffID, parentChainID)
		layersDigest = append(layersDigest, image_puller.LayerDigest{
			BlobID:        layer.BlobID,
			Size:          layer.Size,
			ChainID:       chainID,
			ParentChainID: parentChainID,
		})
		parentChainID = chainID
	}

	return layersDigest
}

func (f *RemoteFetcher) chainID(diffID string, parentChainID string) string {
	diffID = strings.Split(diffID, ":")[1]
	chainID := diffID

	if parentChainID != "" {
		parentChainID = strings.Split(parentChainID, ":")[1]
		chainIDSha := sha256.Sum256([]byte(fmt.Sprintf("%s %s", parentChainID, diffID)))
		chainID = hex.EncodeToString(chainIDSha[:32])
	}

	return "sha256:" + chainID
}
