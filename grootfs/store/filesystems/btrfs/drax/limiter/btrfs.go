package limiter // import "code.cloudfoundry.org/grootfs/store/filesystems/btrfs/drax/limiter"

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"code.cloudfoundry.org/commandrunner"

	"code.cloudfoundry.org/lager"
)

type BtrfsLimiter struct {
	commandRunner commandrunner.CommandRunner
	btrfsBin      string
}

func NewBtrfsLimiter(btrfsBin string, commandRunner commandrunner.CommandRunner) *BtrfsLimiter {
	return &BtrfsLimiter{
		commandRunner: commandRunner,
		btrfsBin:      btrfsBin,
	}
}

func (i *BtrfsLimiter) ApplyDiskLimit(logger lager.Logger, path string, diskLimit int64, exclusiveLimit bool) error {
	logger = logger.Session("btrfs-applying-quotas", lager.Data{"path": path, "diskLimit": diskLimit, "exclusiveLimit": exclusiveLimit})
	logger.Info("start")
	defer logger.Info("end")

	cmd := exec.Command(i.btrfsBin, i.argsForLimit(path, strconv.FormatInt(diskLimit, 10), exclusiveLimit)...)
	combinedBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = combinedBuffer
	cmd.Stderr = combinedBuffer

	logger.Debug("starting-btrfs-command", lager.Data{"cmd": cmd.Path, "args": cmd.Args})
	if err := i.commandRunner.Run(cmd); err != nil {
		logger.Error("command-failed", err, lager.Data{"commandOutput": combinedBuffer.String()})
		return fmt.Errorf(strings.TrimSpace(combinedBuffer.String()))
	}

	return nil
}

func (i *BtrfsLimiter) DestroyQuotaGroup(logger lager.Logger, path string) error {
	logger = logger.Session("btrfs-destroying-qgroup", lager.Data{"path": path})
	logger.Info("start")
	defer logger.Info("end")

	cmd := exec.Command(i.btrfsBin, "qgroup", "destroy", path, path)
	combinedBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = combinedBuffer
	cmd.Stderr = combinedBuffer

	if err := i.commandRunner.Run(cmd); err != nil {
		logger.Error("command-failed", err)
		return fmt.Errorf(strings.TrimSpace(combinedBuffer.String()))
	}

	return nil
}

func (i *BtrfsLimiter) argsForLimit(path, diskLimit string, exclusiveLimit bool) []string {
	args := []string{"qgroup", "limit"}
	if exclusiveLimit {
		args = append(args, "-e")
	}

	return append(args, diskLimit, path)
}
