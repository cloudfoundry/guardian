package loopback

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	errorspkg "github.com/pkg/errors"
)

type LoSetupWrapper struct{}

func NewLoSetup() *LoSetupWrapper {
	return &LoSetupWrapper{}
}

func (l LoSetupWrapper) FindAssociatedLoopDevice(filePath string) (string, error) {
	_, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	errBuffer := bytes.NewBuffer([]byte{})
	cmd := exec.Command("losetup", "-j", filePath)
	cmd.Stderr = errBuffer
	outputBytes, err := cmd.Output()
	if err != nil {
		return "", errorspkg.Wrapf(err, "finding attached loop device: %s", errBuffer.String())
	}

	output := string(outputBytes)
	if len(output) == 0 {
		return "", fmt.Errorf("no loop device associated with path %q", filePath)
	}

	losetupColumns := strings.Split(string(output), ":")
	if len(losetupColumns) == 3 {
		return losetupColumns[0], nil
	}

	return "", errorspkg.Errorf("unexpected losetup output: %s", string(output))
}

func (l LoSetupWrapper) EnableDirectIO(loopdevPath string) error {
	fd, err := os.Open(loopdevPath)
	if err != nil {
		return err
	}
	defer fd.Close()

	const LOOP_SET_DIRECT_IO = uintptr(0x4C08)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd.Fd()), LOOP_SET_DIRECT_IO, uintptr(1))
	if errno != 0 {
		return fmt.Errorf("failed to set direct-io on loop device: errno %d, dev %q", errno, loopdevPath)
	}

	return nil
}
