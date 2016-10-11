package local // import "code.cloudfoundry.org/grootfs/fetcher/local"

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"

	"code.cloudfoundry.org/grootfs/image_puller"
	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var TarBin = "tar"

type LocalFetcher struct {
}

func NewLocalFetcher() *LocalFetcher {
	return &LocalFetcher{}
}

func (l *LocalFetcher) StreamBlob(logger lager.Logger, imageURL *url.URL,
	source string) (io.ReadCloser, int64, error) {
	logger = logger.Session("stream-blob", lager.Data{
		"imageURL": imageURL.String(),
		"source":   source,
	})
	logger.Info("start")
	defer logger.Info("end")

	imagePath := imageURL.String()
	if _, err := os.Stat(imagePath); err != nil {
		return nil, 0, fmt.Errorf("local image not found in `%s` %s", imagePath, err)
	}

	tarCmd := exec.Command(TarBin, "-cp", "-C", imagePath, ".")
	stdoutPipe, err := tarCmd.StdoutPipe()
	if err != nil {
		return nil, 0, fmt.Errorf("creating pipe: %s", err)
	}

	logger.Debug("starting-tar", lager.Data{"path": tarCmd.Path, "args": tarCmd.Args})
	if err := tarCmd.Start(); err != nil {
		return nil, 0, fmt.Errorf("reading local image: %s", err)
	}

	return NewCallbackReader(logger, tarCmd.Wait, stdoutPipe), 0, nil
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
