package layer_fetcher // import "code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/lager"

	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	errorspkg "github.com/pkg/errors"
)

//go:generate counterfeiter . Source
type Source interface {
	Manifest(logger lager.Logger, baseImageURL *url.URL) (Manifest, error)
	Config(logger lager.Logger, baseImageURL *url.URL, manifest Manifest) (specsv1.Image, error)
	Blob(logger lager.Logger, baseImageURL *url.URL, digest string) (string, int64, error)
}

type LayerFetcher struct {
	source      Source
	cacheDriver fetcher.CacheDriver
}

func NewLayerFetcher(source Source, cacheDriver fetcher.CacheDriver) *LayerFetcher {
	return &LayerFetcher{
		source:      source,
		cacheDriver: cacheDriver,
	}
}

func (f *LayerFetcher) BaseImageInfo(logger lager.Logger, baseImageURL *url.URL) (base_image_puller.BaseImageInfo, error) {
	logger = logger.Session("layers-digest", lager.Data{"baseImageURL": baseImageURL})
	logger.Info("starting")
	defer logger.Info("ending")

	logger.Debug("fetching-image-manifest")
	manifest, err := f.source.Manifest(logger, baseImageURL)
	if err != nil {
		return base_image_puller.BaseImageInfo{}, err
	}
	logger.Debug("image-manifest", lager.Data{"manifest": manifest})

	logger.Debug("fetching-image-config")
	contents, _, err := f.cacheDriver.FetchBlob(logger, manifest.ConfigCacheKey,
		func(logger lager.Logger) ([]byte, int64, error) {
			config, err := f.source.Config(logger, baseImageURL, manifest)
			if err != nil {
				return nil, 0, err
			}

			configJSON, err := json.Marshal(config)
			if err != nil {
				return nil, 0, errorspkg.Wrap(err, "encoding config to JSON")
			}

			return configJSON, 0, nil
		},
	)
	if err != nil {
		return base_image_puller.BaseImageInfo{}, err
	}

	var config specsv1.Image
	if err := json.Unmarshal(contents, &config); err != nil {
		return base_image_puller.BaseImageInfo{}, errorspkg.Wrap(err, "decoding config from JSON")
	}
	logger.Debug("image-config", lager.Data{"config": config})

	return base_image_puller.BaseImageInfo{
		LayersDigest: f.createLayersDigest(logger, manifest, config),
		Config:       config,
	}, nil
}

func (f *LayerFetcher) StreamBlob(logger lager.Logger, baseImageURL *url.URL, source string) (io.ReadCloser, int64, error) {
	logger = logger.Session("streaming", lager.Data{"baseImageURL": baseImageURL})
	logger.Info("starting")
	defer logger.Info("ending")

	blobFilePath, size, err := f.source.Blob(logger, baseImageURL, source)
	if err != nil {
		logger.Error("source-blob-failed", err)
		return nil, 0, err
	}

	blobReader, err := NewBlobReader(blobFilePath)
	if err != nil {
		logger.Error("blob-reader-failed", err)
		return nil, 0, err
	}

	return blobReader, size, nil
}

func (f *LayerFetcher) createLayersDigest(logger lager.Logger,
	manifest Manifest, config specsv1.Image,
) []base_image_puller.LayerDigest {
	layersDigest := []base_image_puller.LayerDigest{}

	var parentChainID string
	for i, layer := range manifest.Layers {
		if i == 0 {
			parentChainID = ""
		}

		diffID := config.RootFS.DiffIDs[i]
		chainID := f.chainID(diffID.String(), parentChainID)
		layersDigest = append(layersDigest, base_image_puller.LayerDigest{
			BlobID:        layer.BlobID,
			Size:          layer.Size,
			ChainID:       chainID,
			ParentChainID: parentChainID,
		})
		parentChainID = chainID
	}

	return layersDigest
}

func (f *LayerFetcher) chainID(diffID string, parentChainID string) string {
	diffID = strings.Split(diffID, ":")[1]
	chainID := diffID

	if parentChainID != "" {
		chainIDSha := sha256.Sum256([]byte(fmt.Sprintf("%s %s", parentChainID, diffID)))
		chainID = hex.EncodeToString(chainIDSha[:32])
	}

	return chainID
}
