package integration_test

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Delete", func() {
	var (
		sourceImagePath string
		baseImagePath   string
		image           groot.ImageInfo
	)

	BeforeEach(func() {
		var err error
		integration.SkipIfNonRootAndNotBTRFS(GrootfsTestUid, Driver)
		sourceImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
		var err error
		image, err = Runner.Create(groot.CreateSpec{
			BaseImage: baseImagePath,
			ID:        "random-id",
			Mount:     true,
		})
		Expect(err).ToNot(HaveOccurred())
	})

	It("deletes an existing image", func() {
		Expect(Runner.Delete("random-id")).To(Succeed())
		Expect(image.Path).NotTo(BeAnExistingFile())
	})

	Context("when the store doesn't exist", func() {
		It("logs the image path", func() {
			logBuffer := gbytes.NewBuffer()
			err := Runner.WithStore("/invalid-store").WithStderr(logBuffer).
				Delete("/path/to/random-id")
			Expect(err).ToNot(HaveOccurred())
			Expect(logBuffer).To(gbytes.Say(`"id":"/path/to/random-id"`))
		})
	})

	Context("when a path is provided instead of an ID", func() {
		It("deletes the image by the path", func() {
			Expect(image.Path).To(BeAnExistingFile())
			Expect(Runner.Delete(image.Path)).To(Succeed())
			Expect(image.Path).NotTo(BeAnExistingFile())
		})

		Context("when a path to an image does not exist", func() {
			It("succeeds but logs a warning", func() {
				fakePath := path.Join(StorePath, "images/non_existing")
				Expect(fakePath).NotTo(BeAnExistingFile())

				outBuffer := gbytes.NewBuffer()
				err := Runner.WithStdout(outBuffer).Delete(fakePath)
				Expect(err).NotTo(HaveOccurred())

				Eventually(outBuffer).Should(gbytes.Say("Image `non_existing` not found. Skipping delete."))
			})
		})

		Context("when the path provided doesn't belong to the `--store` provided", func() {
			It("returns an error", func() {
				outBuffer := gbytes.NewBuffer()
				err := Runner.WithStdout(outBuffer).Delete("/Iamnot/in/the/storage/images/1234/rootfs")
				Expect(err).ToNot(HaveOccurred())

				Eventually(outBuffer).Should(gbytes.Say("path `/Iamnot/in/the/storage/images/1234/rootfs` is outside store path"))
			})
		})
	})

	Context("when the image ID doesn't exist", func() {
		It("succeeds but logs a warning", func() {
			outBuffer := gbytes.NewBuffer()
			err := Runner.WithStdout(outBuffer).Delete("non-existing-id")
			Expect(err).NotTo(HaveOccurred())

			Eventually(outBuffer).Should(gbytes.Say("Image `non-existing-id` not found. Skipping delete."))
		})
	})

	Context("when it fails to delete the image", func() {
		var mntPoint string

		JustBeforeEach(func() {
			mntPoint = filepath.Join(image.Path, "mnt")
			Expect(os.Mkdir(mntPoint, 0700)).To(Succeed())
			Expect(syscall.Mount(mntPoint, mntPoint, "none", syscall.MS_BIND, "")).To(Succeed())
		})

		AfterEach(func() {
			Expect(syscall.Unmount(mntPoint, syscall.MNT_DETACH)).To(Succeed())
		})

		It("doesn't remove the metadata file", func() {
			metadataPath := filepath.Join(StorePath, store.MetaDirName, "dependencies", "image:random-id.json")
			Expect(metadataPath).To(BeAnExistingFile())
			Expect(Runner.Delete(image.Path)).To(MatchError(ContainSubstring("deleting image path")))
			Expect(metadataPath).To(BeAnExistingFile())
		})
	})

	Context("when the id is not provided", func() {
		It("fails", func() {
			outBuffer := gbytes.NewBuffer()
			err := Runner.WithStdout(outBuffer).Delete("")
			Expect(err).To(HaveOccurred())

			Eventually(outBuffer).Should(gbytes.Say("id was not specified"))
		})
	})
})
