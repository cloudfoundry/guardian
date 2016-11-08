package groot_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create with local images", func() {
	var baseImagePath string

	BeforeEach(func() {
		var err error
		baseImagePath, err = ioutil.TempDir("", "local-image-dir")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(baseImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
		Expect(os.MkdirAll(path.Join(baseImagePath, "permissive-folder"), 0777)).To(Succeed())

		// we need to explicitly apply perms because mkdir is subject to umask
		Expect(os.Chmod(path.Join(baseImagePath, "permissive-folder"), 0777)).To(Succeed())
	})

	It("creates a root filesystem", func() {
		image, err := integration.CreateImageWSpec(GrootFSBin, StorePath, DraxBin, groot.CreateSpec{
			ID:        "random-id",
			BaseImage: baseImagePath,
		})
		Expect(err).NotTo(HaveOccurred())

		imageContentPath := path.Join(image.RootFSPath, "foo")
		Expect(imageContentPath).To(BeARegularFile())
		fooContents, err := ioutil.ReadFile(imageContentPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(fooContents)).To(Equal("hello-world"))
	})

	It("keeps folders original permissions", func() {
		image, err := integration.CreateImageWSpec(GrootFSBin, StorePath, DraxBin, groot.CreateSpec{
			ID:        "random-id",
			BaseImage: baseImagePath,
		})
		Expect(err).NotTo(HaveOccurred())

		permissiveFolderPath := path.Join(image.RootFSPath, "permissive-folder")
		stat, err := os.Stat(permissiveFolderPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0777)))
	})

	Context("when required args are not provided", func() {
		It("returns an error", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "create")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Out).Should(gbytes.Say("invalid arguments"))
		})
	})

	Context("when image content changes", func() {
		BeforeEach(func() {
			Expect(integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id", 0)).NotTo(BeNil())
		})

		It("uses the new content for the new image", func() {
			Expect(ioutil.WriteFile(path.Join(baseImagePath, "bar"), []byte("this-is-a-bar-content"), 0644)).To(Succeed())

			image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id-2", 0)

			imageContentPath := path.Join(image.RootFSPath, "foo")
			Expect(imageContentPath).To(BeARegularFile())
			barImageContentPath := path.Join(image.RootFSPath, "bar")
			Expect(barImageContentPath).To(BeARegularFile())
		})
	})

	Describe("unpacked volume caching", func() {
		It("caches the unpacked image in a subvolume with snapshots", func() {
			integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id", 0)

			volumeID := integration.BaseImagePathToVolumeID(baseImagePath)
			layerSnapshotPath := filepath.Join(StorePath, CurrentUserID, "volumes", volumeID)
			Expect(ioutil.WriteFile(layerSnapshotPath+"/injected-file", []byte{}, 0666)).To(Succeed())

			image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id-2", 0)
			Expect(path.Join(image.RootFSPath, "foo")).To(BeARegularFile())
			Expect(path.Join(image.RootFSPath, "injected-file")).To(BeARegularFile())
		})
	})

	Context("when local directory does not exist", func() {
		It("returns an error", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", "/invalid/image", "random-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
		})
	})

	Context("when the image has links", func() {
		BeforeEach(func() {
			Expect(os.Symlink(filepath.Join(baseImagePath, "foo"), filepath.Join(baseImagePath, "bar"))).To(Succeed())
		})

		It("unpacks the symlinks", func() {
			image, err := integration.CreateImageWSpec(GrootFSBin, StorePath, DraxBin, groot.CreateSpec{
				ID:        "random-id",
				BaseImage: baseImagePath,
			})
			Expect(err).NotTo(HaveOccurred())

			content, err := ioutil.ReadFile(filepath.Join(image.RootFSPath, "bar"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("hello-world"))
		})
	})
})
