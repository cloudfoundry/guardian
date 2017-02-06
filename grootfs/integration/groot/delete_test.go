package groot_test

import (
	"io/ioutil"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Delete", func() {
	var (
		sourceImagePath string
		baseImagePath   string
		image           groot.Image
	)

	BeforeEach(func() {
		integration.SkipIfNotBTRFS(Driver)

		var err error
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

				Eventually(outBuffer).Should(gbytes.Say("image `non_existing` was not found"))
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

			Eventually(outBuffer).Should(gbytes.Say("image `non-existing-id` was not found"))
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

	Context("when drax is not in PATH", func() {
		It("returns a warning", func() {
			errBuffer := gbytes.NewBuffer()
			outBuffer := gbytes.NewBuffer()
			err := Runner.WithoutDraxBin().
				WithLogLevel(lager.INFO).
				WithEnvVar("PATH=/usr/sbin:/usr/bin:/sbin:/bin").
				WithStdout(outBuffer).
				WithStderr(errBuffer).
				Delete("random-id")
			Expect(err).NotTo(HaveOccurred())
			Eventually(errBuffer).Should(gbytes.Say("could not delete quota group"))
			Eventually(outBuffer).Should(gbytes.Say("Image random-id deleted"))
		})
	})
})
