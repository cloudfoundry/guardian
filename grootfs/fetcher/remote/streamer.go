package remote

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/Sirupsen/logrus"
	"github.com/containers/image/types"
)

type RemoteStreamer struct {
	imgSource types.ImageSource
}

func NewRemoteStreamer(imgSource types.ImageSource) *RemoteStreamer {
	return &RemoteStreamer{
		imgSource: imgSource,
	}
}

func (s *RemoteStreamer) Stream(logger lager.Logger, digest string) (io.ReadCloser, int64, error) {
	logrus.SetOutput(os.Stderr)
	logger = logger.Session("layer-streaming", lager.Data{"digest": digest})
	logger.Info("start")
	defer logger.Info("end")

	stream, size, err := s.imgSource.GetBlob(digest)
	if err != nil {
		return nil, 0, err
	}
	logger.Debug("got-blob-stream", lager.Data{"size": size})

	tarStream, err := gzip.NewReader(stream)
	if err != nil {
		return nil, 0, fmt.Errorf("creating tar reader from gzip: %s", err)
	}

	return tarStream, 0, nil
}
