package groot_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create with local images", func() {
	var (
		sourceImagePath string
		baseImagePath   string
		baseImageFile   *os.File
	)

	BeforeEach(func() {
		var err error
		sourceImagePath, err = ioutil.TempDir("", "local-image-dir")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
		Expect(os.MkdirAll(path.Join(sourceImagePath, "permissive-folder"), 0777)).To(Succeed())

		// we need to explicitly apply perms because mkdir is subject to umask
		Expect(os.Chmod(path.Join(sourceImagePath, "permissive-folder"), 0777)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile = integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
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

	Context("timestamps", func() {
		var modTime time.Time

		BeforeEach(func() {
			location := time.FixedZone("foo", 0)
			modTime = time.Date(2014, 10, 14, 22, 8, 32, 0, location)

			oldFilePath := path.Join(sourceImagePath, "old-file")
			Expect(ioutil.WriteFile(oldFilePath, []byte("hello-world"), 0644)).To(Succeed())
			Expect(os.Chtimes(oldFilePath, time.Now(), modTime)).To(Succeed())
		})

		It("preserves the timestamps", func() {
			image, err := integration.CreateImageWSpec(GrootFSBin, StorePath, DraxBin, groot.CreateSpec{
				ID:        "random-id",
				BaseImage: baseImagePath,
			})
			Expect(err).NotTo(HaveOccurred())

			imageOldFilePath := path.Join(image.RootFSPath, "old-file")
			fi, err := os.Stat(imageOldFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(fi.ModTime().Unix()).To(Equal(modTime.Unix()))
		})
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
		JustBeforeEach(func() {
			Expect(integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id", 0)).NotTo(BeNil())
		})

		It("uses the new content for the new image", func() {
			Expect(ioutil.WriteFile(path.Join(sourceImagePath, "bar"), []byte("this-is-a-bar-content"), 0644)).To(Succeed())
			integration.UpdateBaseImageTar(baseImagePath, sourceImagePath)

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

	Context("when local image does not exist", func() {
		It("returns an error", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", "/invalid/image", "random-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
		})
	})

	Context("when the image has links", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(
				path.Join(sourceImagePath, "symlink-target"), []byte("hello-world"), 0644),
			).To(Succeed())
			Expect(os.Symlink(
				filepath.Join(sourceImagePath, "symlink-target"),
				filepath.Join(sourceImagePath, "symlink"),
			)).To(Succeed())
		})

		It("unpacks the symlinks", func() {
			image, err := integration.CreateImageWSpec(GrootFSBin, StorePath, DraxBin, groot.CreateSpec{
				ID:        "random-id",
				BaseImage: baseImagePath,
			})
			Expect(err).NotTo(HaveOccurred())

			content, err := ioutil.ReadFile(filepath.Join(image.RootFSPath, "symlink"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("hello-world"))
		})

		Context("timestamps", func() {
			BeforeEach(func() {
				cmd := exec.Command("touch", "-h", "-d", "2014-01-01", path.Join(sourceImagePath, "symlink"))
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess.Wait()).Should(gexec.Exit(0))
			})

			It("preserves the timestamps", func() {
				image, err := integration.CreateImageWSpec(GrootFSBin, StorePath, DraxBin, groot.CreateSpec{
					ID:        "random-id",
					BaseImage: baseImagePath,
				})
				Expect(err).NotTo(HaveOccurred())

				symlinkTargetFilePath := path.Join(image.RootFSPath, "symlink-target")
				symlinkTargetFi, err := os.Stat(symlinkTargetFilePath)
				Expect(err).NotTo(HaveOccurred())

				symlinkFilePath := path.Join(image.RootFSPath, "symlink")
				symlinkFi, err := os.Lstat(symlinkFilePath)
				Expect(err).NotTo(HaveOccurred())

				location := time.FixedZone("foo", 0)
				modTime := time.Date(2014, 01, 01, 0, 0, 0, 0, location)
				Expect(symlinkTargetFi.ModTime().Unix()).ToNot(
					Equal(symlinkFi.ModTime().Unix()),
				)
				Expect(symlinkFi.ModTime().Unix()).To(Equal(modTime.Unix()))
			})
		})
	})
})
