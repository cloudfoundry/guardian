package local // import "code.cloudfoundry.org/grootfs/fetcher/local"

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type LocalFetcher struct {
}

func NewLocalFetcher() *LocalFetcher {
	return &LocalFetcher{}
}

func (l *LocalFetcher) StreamBlob(logger lager.Logger, baseImageURL *url.URL,
	source string) (io.ReadCloser, int64, error) {
	logger = logger.Session("stream-blob", lager.Data{
		"baseImageURL": baseImageURL.String(),
		"source":       source,
	})
	logger.Info("start")
	defer logger.Info("end")

	baseImagePath := baseImageURL.String()
	if _, err := os.Stat(baseImagePath); err != nil {
		return nil, 0, fmt.Errorf("local image not found in `%s` %s", baseImagePath, err)
	}

	if err := l.validateBaseImage(baseImagePath); err != nil {
		return nil, 0, fmt.Errorf("invalid base image: %s", err.Error())
	}

	logger.Debug("opening-tar", lager.Data{"baseImagePath": baseImagePath})
	stream, err := os.Open(baseImagePath)
	if err != nil {
		return nil, 0, fmt.Errorf("reading local image: %s", err)
	}

	return stream, 0, nil
}

func (l *LocalFetcher) BaseImageInfo(logger lager.Logger, baseImageURL *url.URL) (base_image_puller.BaseImageInfo, error) {
	logger = logger.Session("layers-digest", lager.Data{"baseImageURL": baseImageURL.String()})
	logger.Info("start")
	defer logger.Info("end")

	stat, err := os.Stat(baseImageURL.String())
	if err != nil {
		return base_image_puller.BaseImageInfo{},
			fmt.Errorf("fetching image timestamp: %s", err)
	}

	return base_image_puller.BaseImageInfo{
		LayersDigest: []base_image_puller.LayerDigest{
			base_image_puller.LayerDigest{
				BlobID:        baseImageURL.String(),
				ParentChainID: "",
				ChainID:       l.generateChainID(baseImageURL.String(), stat.ModTime().UnixNano()),
			},
		},
		Config: specsv1.Image{},
	}, nil
}

func (l *LocalFetcher) generateChainID(baseImagePath string, timestamp int64) string {
	baseImagePathSha := sha256.Sum256([]byte(baseImagePath))
	return fmt.Sprintf("%s-%d", hex.EncodeToString(baseImagePathSha[:32]), timestamp)
}

func (l *LocalFetcher) validateBaseImage(baseImagePath string) error {
	stat, err := os.Stat(baseImagePath)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		return errors.New("directory provided instead of a tar file")
	}

	return nil
}
