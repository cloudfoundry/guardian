package groot_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Stats", func() {
	var (
		sourceImagePath string
		baseImagePath   string
		image           groot.Image
	)

	BeforeEach(func() {
		var err error
		sourceImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
	})

	Context("when the store doesn't exist", func() {
		It("logs the image path", func() {
			logBuffer := gbytes.NewBuffer()
			_, err := Runner.WithStore("/invalid-store").WithStderr(logBuffer).
				Stats("/path/to/random-id")
			Expect(err).To(HaveOccurred())
			Expect(logBuffer).To(gbytes.Say(`"id":"/path/to/random-id"`))
		})
	})

	Context("when image exists", func() {
		var expectedStats groot.VolumeStats

		BeforeEach(func() {
			cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(sourceImagePath, "fatfile")), "bs=1048576", "count=5")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})

		JustBeforeEach(func() {
			image = integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id", 0)
			cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(image.RootFSPath, "hello")), "bs=1048576", "count=4")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
			expectedStats = groot.VolumeStats{
				DiskUsage: groot.DiskUsage{
					TotalBytesUsed:     9453568,
					ExclusiveBytesUsed: 4210688,
				},
			}
		})

		Context("when the last parameter is the image ID", func() {
			It("returns the stats for given image id", func() {
				stats, err := Runner.Stats("random-id")
				Expect(err).NotTo(HaveOccurred())

				Expect(stats).To(Equal(expectedStats))
			})
		})

		Context("when the last parameter is the image path", func() {
			It("returns the stats for given image path", func() {
				stats, err := Runner.Stats(image.Path)
				Expect(err).NotTo(HaveOccurred())

				Expect(stats).To(Equal(expectedStats))
			})
		})
	})

	Context("when the image id doesn't exist", func() {
		Context("when the last parameter is a image id", func() {
			It("returns an error", func() {
				_, err := Runner.Stats("invalid-id")
				Expect(err).To(MatchError(ContainSubstring("image not found: invalid-id")))
			})
		})

		Context("when the last parameter is a path", func() {
			It("returns an error", func() {
				invalidImagePath := filepath.Join(StorePath, CurrentUserID, store.IMAGES_DIR_NAME, "not-here")
				_, err := Runner.Stats(invalidImagePath)
				Expect(err).To(MatchError(ContainSubstring("image not found: not-here")))
			})

			Context("when the path provided doesn't belong to the `--store` provided", func() {
				It("returns an error", func() {
					_, err := Runner.Stats("/Iamnot/in/the/storage/images/1234/rootfs")
					Expect(err).To(MatchError(ContainSubstring("path `/Iamnot/in/the/storage/images/1234/rootfs` is outside store path")))
				})
			})
		})
	})

	Context("when the image id is not provided", func() {
		It("returns an error", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "--drax-bin", DraxBin, "stats")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Out).Should(gbytes.Say("invalid arguments"))
		})
	})
})
