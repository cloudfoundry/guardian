package testhelpers

import (
	"bufio"
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	. "github.com/onsi/gomega"
)

func CleanUpOverlayMounts(mountPath string, storeName string) {
	testStorePath := filepath.Join(mountPath, storeName)

	output, err := exec.Command("mount").Output()
	Expect(err).NotTo(HaveOccurred())

	buffer := bytes.NewBuffer(output)
	scanner := bufio.NewScanner(buffer)
	for scanner.Scan() {
		mountLine := scanner.Text()
		if strings.Contains(mountLine, testStorePath) {
			mountPoint := strings.Split(mountLine, " ")[2]
			Expect(syscall.Unmount(mountPoint, 0)).To(Succeed())
		}
	}
}
