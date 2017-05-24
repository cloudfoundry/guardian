package testhelpers

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/gomega"
)

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
			Expect(syscall.Unmount(mountPoint, 0)).To(Succeed())
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
