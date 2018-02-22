package tar_fetcher // import "code.cloudfoundry.org/grootfs/fetcher/tar_fetcher"

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

type TarFetcher struct {
	baseImagePath string
}

func NewTarFetcher(baseImageURL *url.URL) *TarFetcher {
	return &TarFetcher{baseImagePath: baseImageURL.String()}
}

func (l *TarFetcher) StreamBlob(logger lager.Logger, layerInfo groot.LayerInfo) (io.ReadCloser, int64, error) {
	logger = logger.Session("stream-blob", lager.Data{
		"baseImagePath": l.baseImagePath,
		"source":        layerInfo.BlobID,
	})
	logger.Info("starting")
	defer logger.Info("ending")

	if _, err := os.Stat(l.baseImagePath); err != nil {
		return nil, 0, errorspkg.Wrapf(err, "local image not found in `%s`", l.baseImagePath)
	}

	if err := l.validateBaseImage(); err != nil {
		return nil, 0, errorspkg.Wrap(err, "invalid base image")
	}

	logger.Debug("opening-tar", lager.Data{"baseImagePath": l.baseImagePath})
	stream, err := os.Open(l.baseImagePath)
	if err != nil {
		return nil, 0, errorspkg.Wrap(err, "reading local image")
	}

	return stream, 0, nil
}

func (l *TarFetcher) BaseImageInfo(logger lager.Logger) (groot.BaseImageInfo, error) {
	logger = logger.Session("layers-digest", lager.Data{"baseImagePath": l.baseImagePath})
	logger.Info("starting")
	defer logger.Info("ending")

	stat, err := os.Stat(l.baseImagePath)
	if err != nil {
		return groot.BaseImageInfo{},
			errorspkg.Wrap(err, "fetching image timestamp")
	}

	return groot.BaseImageInfo{
		LayerInfos: []groot.LayerInfo{
			groot.LayerInfo{
				BlobID:        l.baseImagePath,
				ParentChainID: "",
				ChainID:       l.generateChainID(stat.ModTime().UnixNano()),
			},
		},
	}, nil
}

func (l *TarFetcher) Close() error {
	return nil
}

func (l *TarFetcher) generateChainID(timestamp int64) string {
	shaSum := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", l.baseImagePath, timestamp)))
	return hex.EncodeToString(shaSum[:])
}

func (l *TarFetcher) validateBaseImage() error {
	stat, err := os.Stat(l.baseImagePath)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		return errorspkg.New("directory provided instead of a tar file")
	}

	return nil
}
