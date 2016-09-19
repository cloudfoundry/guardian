package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func CreateBundle(grootFSBin, storePath, imagePath, id string, diskLimit int64) groot.Bundle {
	cmd := runner.CreateCmd{
		GrootFSBin: grootFSBin,
		StorePath:  storePath,
		Spec: groot.CreateSpec{
			ID:        id,
			Image:     imagePath,
			DiskLimit: diskLimit,
		},
		LogLevel: lager.DEBUG,
		LogFile:  GinkgoWriter,
	}

	bundlePath, err := cmd.Run()
	Expect(err).NotTo(HaveOccurred())

	return store.NewBundle(bundlePath)
}

func CreateBundleWSpec(grootFSBin, storePath string, spec groot.CreateSpec) (groot.Bundle, error) {
	cmd := runner.CreateCmd{
		GrootFSBin: grootFSBin,
		StorePath:  storePath,
		Spec:       spec,
		LogLevel:   lager.DEBUG,
		LogFile:    GinkgoWriter,
	}

	bundlePath, err := cmd.Run()
	if err != nil {
		return nil, err
	}

	return store.NewBundle(bundlePath), nil
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

type CustomRoundTripper struct {
	RoundTripFn func(req *http.Request) (*http.Response, error)
}

func (r *CustomRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return r.RoundTripFn(req)
}
