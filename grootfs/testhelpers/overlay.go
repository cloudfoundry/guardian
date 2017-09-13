package testhelpers

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func CleanUpExternalLogDevice(externalLogPath string) {
	externalLogPathLoopDevice := bytes.NewBuffer([]byte{})
	getExternalLogDeviceCmd := exec.Command("sh", "-c", fmt.Sprintf("losetup -a | grep %s.external-log | cut -d : -f 1", externalLogPath))
	getExternalLogDeviceCmd.Stdout = io.MultiWriter(GinkgoWriter, externalLogPathLoopDevice)
	getExternalLogDeviceCmd.Stderr = GinkgoWriter
	Expect(getExternalLogDeviceCmd.Run()).To(Succeed())

	cleanExternalLogDeviceCmd := exec.Command("sh", "-c", fmt.Sprintf("losetup -d %s", externalLogPathLoopDevice.String()))
	cleanExternalLogDeviceCmd.Stdout = GinkgoWriter
	cleanExternalLogDeviceCmd.Stderr = GinkgoWriter
	Expect(cleanExternalLogDeviceCmd.Run()).To(Succeed())
}

func CleanUpOverlayMounts(mountPath string) {
	output, err := exec.Command("mount").Output()
	Expect(err).NotTo(HaveOccurred())

	mountPoints := []string{}
	buffer := bytes.NewBuffer(output)
	scanner := bufio.NewScanner(buffer)
	for scanner.Scan() {
		mountLine := scanner.Text()
		mountInfo := strings.Split(mountLine, " ")
		mountType := mountInfo[0]
		if mountType == "overlay" && strings.Contains(mountLine, mountPath) {
			mountPoint := mountInfo[2]
			mountPoints = append(mountPoints, mountPoint)
		}
	}

	for _, point := range mountPoints {
		Expect(ensureCleanUp(point)).To(Succeed())
	}

}

func ensureCleanUp(mountPoint string) error {
	Expect(syscall.Unmount(mountPoint, 0)).To(Succeed())

	var rmErr error
	for i := 0; i < 5; i++ {
		if rmErr = os.RemoveAll(mountPoint); rmErr == nil {
			return nil
		}

		Expect(syscall.Unmount(mountPoint, 0)).To(Succeed())

		time.Sleep(100 * time.Millisecond)
	}

	return rmErr
}

func CleanUpImages(storePath string) {
	files, err := ioutil.ReadDir(filepath.Join(storePath, store.ImageDirName))
	if err != nil {
		return
	}

	for _, fileInfo := range files {
		if fileInfo.IsDir() {
			Expect(os.RemoveAll(filepath.Join(storePath, store.ImageDirName, fileInfo.Name()))).To(Succeed())
		}
	}
}
