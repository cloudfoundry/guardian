package testhelpers

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

func CleanUpOverlayMounts(mountPath string) {
	var mountPoints []string

	for i := 0; i < 10; i++ {
		mountPoints = internalMountPoints(mountPath)
		if len(mountPoints) != 0 {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	for _, point := range mountPoints {
		Expect(ensureCleanUp(point)).To(Succeed())
	}
}

func internalMountPoints(mountPath string) []string {
	output, err := exec.Command("cat", "/proc/mounts").Output()
	Expect(err).NotTo(HaveOccurred())

	mountPoints := []string{}
	buffer := bytes.NewBuffer(output)
	scanner := bufio.NewScanner(buffer)
	for scanner.Scan() {
		mountLine := scanner.Text()
		mountInfo := strings.Split(mountLine, " ")
		mountType := mountInfo[0]
		if mountType == "overlay" && strings.Contains(mountLine, mountPath) {
			mountPoint := mountInfo[1]
			mountPoints = append(mountPoints, mountPoint)
		}
	}

	return mountPoints
}

func log(buff *gbytes.Buffer, message string, args ...interface{}) {
	_, err := buff.Write([]byte(fmt.Sprintf(message, args...)))
	Expect(err).NotTo(HaveOccurred())
}

func ensureCleanUp(mountPoint string) error {
	buff := gbytes.NewBuffer()
	log(buff, "Unmounting overlay mountpoint %s\n", mountPoint)
	Expect(syscall.Unmount(mountPoint, 0)).To(Succeed())

	var rmErr error

	defer func() {
		if rmErr != nil {
			fmt.Println("Ensure cleanup loop failed. Details below:", string(buff.Contents()))
		}
	}()

	for i := 0; i < 5; i++ {
		log(buff, "This is #%d rm attempt of mountPoint %s\n", i+1, mountPoint)
		if rmErr = os.RemoveAll(mountPoint); rmErr == nil {
			return nil
		}

		log(buff, "Unmounting overlay mountpoint %s again\n", mountPoint)
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
