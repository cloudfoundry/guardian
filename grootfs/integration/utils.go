package integration

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func CreateBundle(grootFSBin, storePath, imagePath, id string, diskLimit int64) groot.Bundle {
	return CreateBundleWSpec(grootFSBin, storePath, groot.CreateSpec{
		ID:        id,
		Image:     imagePath,
		DiskLimit: diskLimit,
	})
}

func CreateBundleWSpec(grootFSBin, storePath string, spec groot.CreateSpec) groot.Bundle {
	args := []string{"--store", storePath, "create"}
	for _, mapping := range spec.UIDMappings {
		args = append(args, "--uid-mapping", fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size))
	}
	for _, mapping := range spec.GIDMappings {
		args = append(args, "--gid-mapping", fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size))
	}
	args = append(args, "--disk-limit-size-bytes", strconv.FormatInt(spec.DiskLimit, 10), spec.Image, spec.ID)

	cmd := exec.Command(grootFSBin, args...)
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, 12*time.Second).Should(gexec.Exit(0))

	return store.NewBundle(strings.TrimSpace(string(sess.Out.Contents())))
}

func DeleteBundle(grootFSBin, storePath, id string) string {
	cmd := exec.Command(grootFSBin, "--store", storePath, "delete", id)
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))
	return string(sess.Out.Contents())
}

func FindUID(user string) uint32 {
	sess, err := gexec.Start(exec.Command("id", "-u", user), nil, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))
	i, err := strconv.ParseInt(strings.TrimSpace(string(sess.Out.Contents())), 10, 32)
	Expect(err).NotTo(HaveOccurred())

	return uint32(i)
}

func FindGID(group string) uint32 {
	sess, err := gexec.Start(exec.Command("id", "-g", group), nil, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	i, err := strconv.ParseInt(strings.TrimSpace(string(sess.Out.Contents())), 10, 32)
	Expect(err).NotTo(HaveOccurred())

	return uint32(i)
}

func ImagePathToVolumeID(imagePath string) string {
	stat, err := os.Stat(imagePath)
	Expect(err).ToNot(HaveOccurred())

	imagePathSha := sha256.Sum256([]byte(imagePath))
	return fmt.Sprintf("%s-%d", hex.EncodeToString(imagePathSha[:32]), stat.ModTime().UnixNano())
}

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

func CleanUpSubvolumes(btrfsPath string, storeName string) {
	for _, subvolumePath := range subvolumePaths(btrfsPath) {
		if !strings.HasPrefix(subvolumePath, storeName+"/") {
			continue
		}

		subvolumeAbsPath := filepath.Join(btrfsPath, subvolumePath)
		subvolumeDelete(subvolumeAbsPath)
	}
}
