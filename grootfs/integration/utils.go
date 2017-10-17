package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func CreateBaseImage(rootUID, rootGID, grootUID, grootGID int) string {
	sourceImagePath, err := ioutil.TempDir("", "")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Chown(sourceImagePath, rootUID, rootGID)).To(Succeed())
	Expect(os.Chmod(sourceImagePath, 0755)).To(Succeed())

	grootFilePath := path.Join(sourceImagePath, "foo")
	Expect(ioutil.WriteFile(grootFilePath, []byte("hello-world"), 0644)).To(Succeed())
	Expect(os.Chown(grootFilePath, grootUID, grootGID)).To(Succeed())

	grootFolder := path.Join(sourceImagePath, "groot-folder")
	Expect(os.Mkdir(grootFolder, 0777)).To(Succeed())
	Expect(os.Chown(grootFolder, grootUID, grootGID)).To(Succeed())
	Expect(ioutil.WriteFile(path.Join(grootFolder, "hello"), []byte("hello-world"), 0644)).To(Succeed())

	rootFilePath := path.Join(sourceImagePath, "bar")
	Expect(ioutil.WriteFile(rootFilePath, []byte("hello-world"), 0644)).To(Succeed())

	rootFolder := path.Join(sourceImagePath, "root-folder")
	Expect(os.Mkdir(rootFolder, 0777)).To(Succeed())
	Expect(ioutil.WriteFile(path.Join(rootFolder, "hello"), []byte("hello-world"), 0644)).To(Succeed())

	grootLinkToRootFile := path.Join(sourceImagePath, "groot-link")
	Expect(os.Symlink(rootFilePath, grootLinkToRootFile)).To(Succeed())
	Expect(os.Lchown(grootLinkToRootFile, grootUID, grootGID))

	return sourceImagePath
}

func CreateBaseImageTar(sourcePath string) *os.File {
	baseImageFile, err := ioutil.TempFile("", "image.tar")
	Expect(err).NotTo(HaveOccurred())
	UpdateBaseImageTar(baseImageFile.Name(), sourcePath)
	return baseImageFile
}

func UpdateBaseImageTar(tarPath, sourcePath string) {
	sess, err := gexec.Start(exec.Command("tar", "-cpf", tarPath, "-C", sourcePath, "."), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, 15*time.Second).Should(gexec.Exit(0))
	Expect(os.Chmod(tarPath, 0666)).To(Succeed())
}

func FindUID(user string) uint32 {
	sess, err := gexec.Start(exec.Command("id", "-u", user), nil, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, 10*time.Second).Should(gexec.Exit(0))
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

func SkipIfNotBTRFS(driver string) {
	if driver != "btrfs" {
		Skip("These tests are only for BTRFS driver. Skipping.")
	}
}

func SkipIfNotXFS(driver string) {
	if driver != "overlay-xfs" {
		Skip("These tests are only for Overlay-XFS driver. Skipping.")
	}
}

func SkipIfNonRoot(uid int) {
	if uid != 0 {
		Skip("These tests can only run as root user. Skipping.")
	}
}

func SkipIfRoot(uid int) {
	if uid == 0 {
		Skip("These tests can only run as non-root users. Skipping.")
	}
}

func CreateFakeDrax() (string, *os.File, *os.File) {
	tempFolder, bin, binCalledFile := CreateFakeBin("drax")
	testhelpers.SuidBinary(bin.Name())
	return tempFolder, bin, binCalledFile
}

func CreateFakeTardis() (string, *os.File, *os.File) {
	tempFolder, bin, binCalledFile := CreateFakeBin("tardis")
	testhelpers.SuidBinary(bin.Name())
	return tempFolder, bin, binCalledFile
}

func CreateFakeBin(binaryName string) (string, *os.File, *os.File) {
	binCalledFile, err := ioutil.TempFile("", "bin-called")
	Expect(err).NotTo(HaveOccurred())
	Expect(binCalledFile.Close()).To(Succeed())
	Expect(os.Chmod(binCalledFile.Name(), 0666)).To(Succeed())

	tempFolder, err := ioutil.TempDir("", "")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Chmod(tempFolder, 0755)).To(Succeed())

	bin, err := os.Create(path.Join(tempFolder, binaryName))
	Expect(err).NotTo(HaveOccurred())
	_, err = bin.WriteString("#!/bin/bash\necho -n \"I'm groot - " + binaryName + "\" > " + binCalledFile.Name())
	Expect(err).NotTo(HaveOccurred())
	Expect(bin.Chmod(0777)).To(Succeed())
	Expect(bin.Close()).To(Succeed())

	return tempFolder, bin, binCalledFile
}

func BaseImagePathToVolumeID(baseImagePath string) string {
	stat, err := os.Stat(baseImagePath)
	Expect(err).ToNot(HaveOccurred())

	baseImagePathSha := sha256.Sum256([]byte(baseImagePath))
	return fmt.Sprintf("%s-%d", hex.EncodeToString(baseImagePathSha[:32]), stat.ModTime().UnixNano())
}

type CustomRoundTripper struct {
	RoundTripFn func(req *http.Request) (*http.Response, error)
}

func (r *CustomRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return r.RoundTripFn(req)
}

func MakeBinaryAccessibleToEveryone(binaryPath string) string {
	binaryName := path.Base(binaryPath)
	tempDir := fmt.Sprintf("/tmp/temp-%s-%d", binaryName, rand.Int())
	Expect(os.MkdirAll(tempDir, 0755)).To(Succeed())
	Expect(os.Chmod(tempDir, 0755)).To(Succeed())

	newBinaryPath := filepath.Join(tempDir, binaryName)
	Expect(os.Rename(binaryPath, newBinaryPath)).To(Succeed())
	Expect(os.Chmod(newBinaryPath, 0755)).To(Succeed())

	return newBinaryPath
}

func String2URL(s string) *url.URL {
	url, err := url.Parse(s)
	Expect(err).NotTo(HaveOccurred())
	return url
}
