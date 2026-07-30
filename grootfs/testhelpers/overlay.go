package testhelpers

import (
	"bufio"
	"bytes"
	"fmt"
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

	// There should be no images in the store
	Expect(len(internalMountPoints(mountPath))).To(Equal(0), fmt.Sprintf("%+v", internalMountPoints(mountPath)))

	// Invalid argument means that the mount path is not a mountpoint, so... whatever
	Expect(syscall.Unmount(mountPath, 0)).To(Or(Not(HaveOccurred()), MatchError("invalid argument"), MatchError("no such file or directory")))
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

func XFSMountPoints() []string {
	output, err := exec.Command("cat", "/proc/mounts").Output()
	Expect(err).NotTo(HaveOccurred())

	mountPoints := []string{}
	buffer := bytes.NewBuffer(output)
	scanner := bufio.NewScanner(buffer)
	for scanner.Scan() {
		mountLine := scanner.Text()
		mountInfo := strings.Split(mountLine, " ")
		mountType := mountInfo[2]
		if mountType == "xfs" {
			mountPoints = append(mountPoints, mountInfo[1])
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
	files, err := os.ReadDir(filepath.Join(storePath, store.ImageDirName))
	if err != nil {
		return
	}

	for _, fileInfo := range files {
		if fileInfo.IsDir() {
			Expect(os.RemoveAll(filepath.Join(storePath, store.ImageDirName, fileInfo.Name()))).To(Succeed())
		}
	}
}
