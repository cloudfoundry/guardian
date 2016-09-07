package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/grootfs/image_puller"
	"code.cloudfoundry.org/lager"

	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

//go:generate counterfeiter . Source
type Source interface {
	Manifest(logger lager.Logger, imageURL *url.URL) (specsv1.Manifest, error)
	Config(logger lager.Logger, imageURL *url.URL, configDigest string) (specsv1.Image, error)
	StreamBlob(logger lager.Logger, imageURL *url.URL, digest string) (io.ReadCloser, int64, error)
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
	configStream, err := f.cacheDriver.Blob(logger, manifest.Config.Digest,
		func(logger lager.Logger) (io.ReadCloser, error) {
			config, err := f.source.Config(logger, imageURL, manifest.Config.Digest)
			if err != nil {
				return nil, err
			}

			rEnd, wEnd, err := os.Pipe()
			if err != nil {
				return nil, fmt.Errorf("making pipe: %s", err)
			}

			if err := json.NewEncoder(wEnd).Encode(config); err != nil {
				return nil, fmt.Errorf("encoding config to JSON: %s", err)
			}

			return rEnd, nil
		},
	)
	if err != nil {
		return image_puller.ImageInfo{}, err
	}

	var config specsv1.Image
	if err := json.NewDecoder(configStream).Decode(&config); err != nil {
		return image_puller.ImageInfo{}, fmt.Errorf("decoding config from JSON: %s", err)
	}
	logger.Debug("image-config", lager.Data{"config": config})

	return image_puller.ImageInfo{
		LayersDigest: f.createLayersDigest(logger, manifest, config),
		Config:       config,
	}, nil
}

func (f *RemoteFetcher) StreamBlob(logger lager.Logger, imageURL *url.URL, source string) (io.ReadCloser, int64, error) {
	logger = logger.Session("streaming", lager.Data{"imageURL": imageURL})
	logger.Info("start")
	defer logger.Info("end")

	stream, err := f.cacheDriver.Blob(logger, source,
		func(logger lager.Logger) (io.ReadCloser, error) {
			stream, _, err := f.source.StreamBlob(logger, imageURL, source)
			if err != nil {
				return nil, err
			}

			return stream, nil
		},
	)
	if err != nil {
		return nil, 0, err
	}

	return stream, 0, nil
}

func (f *RemoteFetcher) createLayersDigest(logger lager.Logger,
	manifest specsv1.Manifest, config specsv1.Image,
) []image_puller.LayerDigest {
	layersDigest := []image_puller.LayerDigest{}

	var parentChainID string
	for i, blobDesc := range manifest.Layers {
		if i == 0 {
			parentChainID = ""
		}

		diffID := config.RootFS.DiffIDs[i]
		chainID := f.chainID(diffID, parentChainID)
		layersDigest = append(layersDigest, image_puller.LayerDigest{
			BlobID:        blobDesc.Digest,
			DiffID:        diffID,
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
