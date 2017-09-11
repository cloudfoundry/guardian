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

	buffer := bytes.NewBuffer(output)
	scanner := bufio.NewScanner(buffer)
	for scanner.Scan() {
		mountLine := scanner.Text()
		mountInfo := strings.Split(mountLine, " ")
		mountType := mountInfo[0]
		if mountType == "overlay" && strings.Contains(mountLine, mountPath) {
			mountPoint := mountInfo[2]
			strace := exec.Command("strace", "-tt", "umount", mountPoint)
			strace.Stderr = GinkgoWriter
			err := strace.Run()
			Expect(err).NotTo(HaveOccurred())
		}
	}
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
