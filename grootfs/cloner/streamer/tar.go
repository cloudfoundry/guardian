package streamer

import (
	"fmt"
	"io"
	"os/exec"

	"code.cloudfoundry.org/lager"
)

var TarBin = "tar"

type TarStreamer struct{}

func NewTarStreamer() *TarStreamer {
	return &TarStreamer{}
}

func (tr *TarStreamer) Stream(logger lager.Logger, source string) (io.ReadCloser, int64, error) {
	logger = logger.Session("tar-streaming", lager.Data{"source": source})
	logger.Debug("start")
	defer logger.Debug("end")

	tarCmd := exec.Command(TarBin, "-cp", "-C", source, ".")
	stdoutPipe, err := tarCmd.StdoutPipe()
	if err != nil {
		return nil, 0, fmt.Errorf("creating pipe: %s", err)
	}

	if err := tarCmd.Start(); err != nil {
		return nil, 0, fmt.Errorf("starting command: %s", err)
	}

	return NewCommandReader(nil, tarCmd.Wait, stdoutPipe), 0, nil
}
