package local

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"

	"code.cloudfoundry.org/grootfs/image_puller"
	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type LocalFetcher struct {
	streamer image_puller.Streamer
}

func NewLocalFetcher(streamer image_puller.Streamer) *LocalFetcher {
	return &LocalFetcher{
		streamer: streamer,
	}
}

func (l *LocalFetcher) Streamer(logger lager.Logger, imageURL *url.URL) (image_puller.Streamer, error) {
	logger = logger.Session("streamer", lager.Data{"image-url": imageURL.String()})
	logger.Info("start")
	defer logger.Info("end")

	_, err := os.Stat(imageURL.String())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("image source does not exist: %s", err)
		}
		return nil, fmt.Errorf("failed to access image path: %s", err)
	}

	return l.streamer, nil
}

func (l *LocalFetcher) ImageInfo(logger lager.Logger, imageURL *url.URL) (image_puller.ImageInfo, error) {
	logger = logger.Session("layers-digest", lager.Data{"image-url": imageURL.String()})
	logger.Info("start")
	defer logger.Info("end")

	stat, err := os.Stat(imageURL.String())
	if err != nil {
		return image_puller.ImageInfo{},
			fmt.Errorf("fetching image timestamp: %s", err)
	}

	return image_puller.ImageInfo{
		LayersDigest: []image_puller.LayerDigest{
			image_puller.LayerDigest{
				BlobID:        imageURL.String(),
				ParentChainID: "",
				ChainID:       l.generateChainID(imageURL.String(), stat.ModTime().UnixNano()),
			},
		},
		Config: specsv1.Image{},
	}, nil
}

func (l *LocalFetcher) generateChainID(imagePath string, timestamp int64) string {
	imagePathSha := sha256.Sum256([]byte(imagePath))
	return fmt.Sprintf("%s-%d", hex.EncodeToString(imagePathSha[:32]), timestamp)
}
