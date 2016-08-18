package streamer

import (
	"fmt"
	"io"
	"os"
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

	if _, err := os.Stat(source); err != nil {
		return nil, 0, fmt.Errorf("source image not found: `%s` %s", source, err)
	}

	tarCmd := exec.Command(TarBin, "-cp", "-C", source, ".")
	stdoutPipe, err := tarCmd.StdoutPipe()
	if err != nil {
		return nil, 0, fmt.Errorf("creating pipe: %s", err)
	}

	logger.Debug("starting-tar", lager.Data{"path": tarCmd.Path, "args": tarCmd.Args})
	if err := tarCmd.Start(); err != nil {
		return nil, 0, fmt.Errorf("starting command: %s", err)
	}

	return NewCallbackReader(logger, tarCmd.Wait, stdoutPipe), 0, nil
}
