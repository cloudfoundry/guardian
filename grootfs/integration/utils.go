package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func CreateBaseImageTar(sourcePath string) *os.File {
	baseImageFile, err := ioutil.TempFile("", "image.tar")
	Expect(err).NotTo(HaveOccurred())
	UpdateBaseImageTar(baseImageFile.Name(), sourcePath)
	return baseImageFile
}

func UpdateBaseImageTar(tarPath, sourcePath string) {
	sess, err := gexec.Start(exec.Command("tar", "-cpf", tarPath, "-C", sourcePath, "."), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))
	Expect(os.Chmod(tarPath, 0666)).To(Succeed())
}

func CreateImage(grootFSBin, storePath, draxBin, baseImagePath, id string, diskLimit int64) groot.Image {
	spec := groot.CreateSpec{
		ID:        id,
		BaseImage: baseImagePath,
		DiskLimit: diskLimit,
	}

	image, err := CreateImageWSpec(grootFSBin, storePath, draxBin, spec)
	Expect(err).NotTo(HaveOccurred())

	return image
}

func CreateImageWSpec(grootFSBin, storePath, draxBin string, spec groot.CreateSpec) (groot.Image, error) {
	runner := &runner.Runner{
		GrootFSBin: grootFSBin,
		StorePath:  storePath,
		DraxBin:    draxBin,
		LogLevel:   lager.DEBUG,
		Stderr:     GinkgoWriter,
	}

	return runner.Create(spec)
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

func CreateFakeDrax() (string, *os.File, *os.File) {
	tempFolder, bin, binCalledFile := CreateFakeBin("drax")
	testhelpers.SuidDrax(bin.Name())
	return tempFolder, bin, binCalledFile
}

func CreateFakeBin(binaryName string) (string, *os.File, *os.File) {
	binCalledFile, err := ioutil.TempFile("", "bin-called")
	Expect(err).NotTo(HaveOccurred())
	binCalledFile.Close()

	tempFolder, err := ioutil.TempDir("", "")
	bin, err := os.Create(path.Join(tempFolder, binaryName))
	Expect(err).NotTo(HaveOccurred())
	bin.WriteString("#!/bin/bash\necho -n \"I'm groot - " + binaryName + "\" > " + binCalledFile.Name())
	bin.Chmod(0777)
	bin.Close()

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
