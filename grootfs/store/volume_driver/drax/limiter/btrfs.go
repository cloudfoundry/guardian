package limiter

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner"
)

type BtrfsLimiter struct {
	commandRunner command_runner.CommandRunner
}

func NewBtrfsLimiter(commandRunner command_runner.CommandRunner) *BtrfsLimiter {
	return &BtrfsLimiter{
		commandRunner: commandRunner,
	}
}

func (i *BtrfsLimiter) ApplyDiskLimit(logger lager.Logger, path string, diskLimit int64) error {
	logger = logger.Session("btrfs-appling-quotas", lager.Data{"path": path})
	logger.Info("start")
	defer logger.Info("end")

	cmd := exec.Command("btrfs", "qgroup", "limit", strconv.FormatInt(diskLimit, 10), path)
	combinedBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = combinedBuffer
	cmd.Stderr = combinedBuffer

	if err := i.commandRunner.Run(cmd); err != nil {
		logger.Error("command-failed", err)
		return fmt.Errorf(strings.TrimSpace(combinedBuffer.String()))
	}

	return nil
}

func (i *BtrfsLimiter) DestroyQuotaGroup(logger lager.Logger, path string) error {
	logger = logger.Session("btrfs-destroying-qgroup", lager.Data{"path": path})
	logger.Info("start")
	defer logger.Info("end")

	cmd := exec.Command("btrfs", "qgroup", "destroy", path, path)
	combinedBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = combinedBuffer
	cmd.Stderr = combinedBuffer

	if err := i.commandRunner.Run(cmd); err != nil {
		logger.Error("command-failed", err)
		return fmt.Errorf(strings.TrimSpace(combinedBuffer.String()))
	}

	return nil
}
