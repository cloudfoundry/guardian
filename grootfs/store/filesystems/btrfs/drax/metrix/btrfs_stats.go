package metrix // import "code.cloudfoundry.org/grootfs/store/filesystems/btrfs/drax/metrix"

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/lager"
)

type BtrfsStats struct {
	commandRunner commandrunner.CommandRunner
	btrfsBin      string
}

func NewBtrfsStats(btrfsBin string, commandRunner commandrunner.CommandRunner) *BtrfsStats {
	return &BtrfsStats{
		commandRunner: commandRunner,
		btrfsBin:      btrfsBin,
	}
}

func (m *BtrfsStats) VolumeStats(logger lager.Logger, path string, forceSync bool) ([]byte, error) {
	logger = logger.Session("btrfs-fetching-volume-stats", lager.Data{"path": path, "forceSync": forceSync})
	logger.Info("start")
	defer logger.Info("end")

	if err := m.isSubvolume(logger, path); err != nil {
		return nil, err
	}

	if forceSync {
		cmd := exec.Command(m.btrfsBin, "filesystem", "sync", path)
		combinedBuffer := bytes.NewBuffer([]byte{})
		cmd.Stdout = combinedBuffer
		cmd.Stderr = combinedBuffer

		if err := m.commandRunner.Run(cmd); err != nil {
			logger.Error("command-failed", err)
			return nil, fmt.Errorf("syncing filesystem: %s", strings.TrimSpace(combinedBuffer.String()))
		}
	}

	cmd := exec.Command(m.btrfsBin, "qgroup", "show", "--raw", "-F", path)
	outputBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = outputBuffer
	errorBuffer := bytes.NewBuffer([]byte{})
	cmd.Stderr = errorBuffer

	if err := m.commandRunner.Run(cmd); err != nil {
		logger.Error("command-failed", err)
		return nil, fmt.Errorf("qgroup usage: %s, %s",
			strings.TrimSpace(outputBuffer.String()),
			strings.TrimSpace(errorBuffer.String()))
	}

	logger.Debug("btrfs-output", lager.Data{"output": outputBuffer.String()})
	return outputBuffer.Bytes(), nil
}

func (m *BtrfsStats) isSubvolume(logger lager.Logger, path string) error {
	cmd := exec.Command(m.btrfsBin, "subvolume", "show", path)
	combinedBuffer := bytes.NewBuffer([]byte{})
	cmd.Stderr = combinedBuffer
	cmd.Stdout = combinedBuffer

	if err := m.commandRunner.Run(cmd); err != nil {
		if strings.Contains(combinedBuffer.String(), "not a subvolume") {
			return fmt.Errorf("`%s` is not a btrfs volume: %s", path, err)
		}

		logger.Error("command-failed", err)
		return fmt.Errorf("checking if volume exists: %s", combinedBuffer.String())
	}

	return nil
}
