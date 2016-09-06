package metrix

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner"
)

type BtrfsMetrics struct {
	commandRunner command_runner.CommandRunner
}

func NewBtrfsMetrics(commandRunner command_runner.CommandRunner) *BtrfsMetrics {
	return &BtrfsMetrics{
		commandRunner: commandRunner,
	}
}

func (m *BtrfsMetrics) VolumeMetrics(logger lager.Logger, path string, forceSync bool) ([]byte, error) {
	logger = logger.Session("btrfs-fetching-volume-metrics", lager.Data{"path": path, "forceSync": forceSync})
	logger.Info("start")
	defer logger.Info("end")

	if err := m.isSubvolume(logger, path); err != nil {
		return nil, err
	}

	if forceSync {
		cmd := exec.Command("btrfs", "filesystem", "sync", path)
		combinedBuffer := bytes.NewBuffer([]byte{})
		cmd.Stdout = combinedBuffer
		cmd.Stderr = combinedBuffer

		if err := m.commandRunner.Run(cmd); err != nil {
			logger.Error("command-failed", err)
			return nil, fmt.Errorf("syncing filesystem: %s", strings.TrimSpace(combinedBuffer.String()))
		}
	}

	cmd := exec.Command("btrfs", "qgroup", "show", "--raw", "-F", path)
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

	return outputBuffer.Bytes(), nil
}

func (m *BtrfsMetrics) isSubvolume(logger lager.Logger, path string) error {
	cmd := exec.Command("btrfs", "subvolume", "show", path)
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
