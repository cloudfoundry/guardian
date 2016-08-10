package unpacker

import (
	"fmt"
	"os"
	"os/exec"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/lager"
)

type TarUnpacker struct {
}

func NewTarUnpacker() *TarUnpacker {
	return &TarUnpacker{}
}

func (u *TarUnpacker) Unpack(logger lager.Logger, spec cloner.UnpackSpec) error {
	logger = logger.Session("unpacking-with-tar", lager.Data{"spec": spec})
	logger.Debug("start")
	defer logger.Debug("end")

	if _, err := os.Stat(spec.RootFSPath); err != nil {
		if err := os.Mkdir(spec.RootFSPath, 0755); err != nil {
			return fmt.Errorf("making destination directory `%s`: %s", spec.RootFSPath, err)
		}
	}

	cmd := exec.Command("tar", "-xp", "-C", spec.RootFSPath)
	cmd.Stdin = spec.Stream
	if err := u.runAndLog(cmd); err != nil {
		return err
	}

	return nil
}

func (u *TarUnpacker) runAndLog(cmd *exec.Cmd) error {
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %s", err, string(output))
}
