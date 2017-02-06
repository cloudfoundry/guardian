package testhelpers

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func subvolumePaths(btrfsPath string) []string {
	cmd := exec.Command("sudo", "btrfs", "subvolume", "list", btrfsPath)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = io.MultiWriter(stdoutBuffer, GinkgoWriter)
	cmd.Stderr = GinkgoWriter
	Expect(cmd.Run()).To(Succeed())

	res := []string{}
	scanner := bufio.NewScanner(strings.NewReader(stdoutBuffer.String()))
	for scanner.Scan() {
		subvolumeFields := strings.Fields(scanner.Text())
		res = append(res, subvolumeFields[len(subvolumeFields)-1])
	}

	return res
}

func subvolumeDelete(subvolumeAbsPath string) {
	_, err := os.Stat(subvolumeAbsPath)
	if err != nil && os.IsNotExist(err) {
		return
	}

	cmd := exec.Command("sudo", "btrfs", "subvolume", "delete", subvolumeAbsPath)
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	Expect(cmd.Run()).To(Succeed(), fmt.Sprintf("deleting volume `%s`", subvolumeAbsPath))
}

func CleanUpBtrfsSubvolumes(btrfsPath string, storeName string) {
	for _, subvolumePath := range subvolumePaths(btrfsPath) {
		if !strings.HasPrefix(subvolumePath, storeName+"/") {
			continue
		}

		subvolumeAbsPath := filepath.Join(btrfsPath, subvolumePath)
		subvolumeDelete(subvolumeAbsPath)
	}
}
