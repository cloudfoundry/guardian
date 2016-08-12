package unpacker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	cmd := exec.Command("tar", "--exclude", "dev/*", "-xp", "-C", spec.RootFSPath)
	cmd.Stdin = spec.Stream
	if err := u.runAndLog(cmd); err != nil {
		return err
	}

	return u.cleanWhiteout(logger, spec.RootFSPath)
}

func (u *TarUnpacker) runAndLog(cmd *exec.Cmd) error {
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %s", err, string(output))
}

func (u *TarUnpacker) cleanWhiteout(logger lager.Logger, rootFSPath string) error {
	toBeDeleted := []string{}

	if err := filepath.Walk(rootFSPath, func(entryPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasPrefix(info.Name(), ".wh.") {
			tbdPath := strings.Replace(entryPath, ".wh.", "", 1)
			toBeDeleted = append(toBeDeleted, entryPath, tbdPath)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("walking the root file system: %s", err)
	}
	logger.Debug("got-enties-to-be-deleted", lager.Data{"toBeDeleted": toBeDeleted})

	for _, path := range toBeDeleted {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("deleting whiteout path: %s", err)
		}
	}

	return nil
}
