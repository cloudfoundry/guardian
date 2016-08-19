package groot_test

import (
	"io/ioutil"
	"os/exec"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create with local images", func() {
	var imagePath string

	BeforeEach(func() {
		var err error
		imagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		Expect(ioutil.WriteFile(path.Join(imagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
	})

	It("creates a root filesystem", func() {
		bundle := integration.CreateBundle(GrootFSBin, StorePath, imagePath, "random-id")
		bundleContentPath := path.Join(bundle.RootFSPath(), "foo")
		Expect(bundleContentPath).To(BeARegularFile())
		fooContents, err := ioutil.ReadFile(bundleContentPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(fooContents)).To(Equal("hello-world"))
	})

	Describe("unpacked volume caching", func() {
		It("caches the unpacked image in a subvolume with snapshots", func() {
			integration.CreateBundle(GrootFSBin, StorePath, imagePath, "random-id")

			volumeID := integration.ImagePathToVolumeID(imagePath)
			layerSnapshotPath := filepath.Join(StorePath, "volumes", volumeID)
			Expect(ioutil.WriteFile(layerSnapshotPath+"/injected-file", []byte{}, 0666)).To(Succeed())

			bundle := integration.CreateBundle(GrootFSBin, StorePath, imagePath, "random-id-2")
			Expect(path.Join(bundle.RootFSPath(), "foo")).To(BeARegularFile())
			Expect(path.Join(bundle.RootFSPath(), "injected-file")).To(BeARegularFile())
		})
	})

	Context("when local directory does not exist", func() {
		It("returns an error", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", "--image", "/invalid/image", "random-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
		})
	})
})
