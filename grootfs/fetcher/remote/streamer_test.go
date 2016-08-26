package remote_test

import (
	"io/ioutil"
	"os"
	"os/exec"

	"code.cloudfoundry.org/grootfs/fetcher/remote"
	"code.cloudfoundry.org/lager/lagertest"

	"github.com/containers/image/docker"
	"github.com/containers/image/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Streamer", func() {
	var (
		tmpDir   string
		logger   *lagertest.TestLogger
		streamer *remote.RemoteStreamer
	)

	BeforeEach(func() {
		var err error
		logger = lagertest.NewTestLogger("Streamer")
		tmpDir, err = ioutil.TempDir("", "streamer-test")
		Expect(err).NotTo(HaveOccurred())

		streamer = remote.NewRemoteStreamer(newImageSource("//cfgarden/empty"))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("Stream", func() {
		It("streams a blob", func() {
			digest := "sha256:6c1f4533b125f8f825188c4f4ff633a338cfce0db2813124d3d518028baf7d7a"
			stream, _, err := streamer.Stream(logger, digest)
			Expect(err).NotTo(HaveOccurred())

			buffer := gbytes.NewBuffer()
			cmd := exec.Command("tar", "tv")
			cmd.Stdin = stream
			sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(buffer).Should(gbytes.Say("hello"))
			Eventually(sess).Should(gexec.Exit(0))
		})

		Context("when fails to get the layer", func() {
			It("returns an error", func() {
				digest := "sha256:invalid-digest"
				_, _, err := streamer.Stream(logger, digest)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

func newImageSource(source string) types.ImageSource {
	ref, err := docker.ParseReference(source)
	Expect(err).NotTo(HaveOccurred())

	imgSource, err := ref.NewImageSource("", true)
	Expect(err).NotTo(HaveOccurred())

	return imgSource
}
