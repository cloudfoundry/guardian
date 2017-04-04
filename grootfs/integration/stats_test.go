package integration_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
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
		image           groot.ImageInfo
		imageID         string
	)

	BeforeEach(func() {
		var err error
		sourceImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		imageID = fmt.Sprintf("random-id-%d", rand.Int())
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
		var (
			expectedStats groot.VolumeStats
			diskLimit     int64
		)

		BeforeEach(func() {
			diskLimit = 1024 * 1024 * 50
			cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(sourceImagePath, "fatfile")), "bs=1048576", "count=5")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})

		JustBeforeEach(func() {
			var err error
			image, err = Runner.Create(groot.CreateSpec{
				BaseImage: baseImagePath,
				ID:        imageID,
				DiskLimit: diskLimit,
				Mount:     true,
			})
			Expect(err).ToNot(HaveOccurred())
			cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(image.Rootfs, "hello")), "bs=1048576", "count=4")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
			expectedStats = groot.VolumeStats{
				DiskUsage: groot.DiskUsage{
					TotalBytesUsed:     9441280,
					ExclusiveBytesUsed: 4198400,
				},
			}
		})

		Context("when the last parameter is the image ID", func() {
			It("returns the stats for given image id", func() {
				stats, err := Runner.Stats(imageID)
				Expect(err).NotTo(HaveOccurred())

				Expect(stats.DiskUsage.TotalBytesUsed).To(
					BeNumerically("~", expectedStats.DiskUsage.TotalBytesUsed, 100),
				)
				Expect(stats.DiskUsage.ExclusiveBytesUsed).To(
					BeNumerically("~", expectedStats.DiskUsage.ExclusiveBytesUsed, 100),
				)
			})
		})

		Context("when the last parameter is the image path", func() {
			It("returns the stats for given image path", func() {
				stats, err := Runner.Stats(image.Path)
				Expect(err).NotTo(HaveOccurred())

				Expect(stats.DiskUsage.TotalBytesUsed).To(
					BeNumerically("~", expectedStats.DiskUsage.TotalBytesUsed, 100),
				)
				Expect(stats.DiskUsage.ExclusiveBytesUsed).To(
					BeNumerically("~", expectedStats.DiskUsage.ExclusiveBytesUsed, 100),
				)
			})
		})

		Context("when the image has no quotas", func() {
			BeforeEach(func() {
				integration.SkipIfNotXFS(Driver)
				diskLimit = 0
			})

			It("returns 0 as exclusive usage", func() {
				stats, err := Runner.Stats(imageID)
				Expect(err).NotTo(HaveOccurred())

				Expect(stats.DiskUsage.TotalBytesUsed).To(
					BeNumerically("~", expectedStats.DiskUsage.TotalBytesUsed-expectedStats.DiskUsage.ExclusiveBytesUsed, 100),
				)
				Expect(stats.DiskUsage.ExclusiveBytesUsed).To(
					BeNumerically("~", 0, 100),
				)
			})
		})
	})

	Context("when the image id doesn't exist", func() {
		Context("when the last parameter is a image id", func() {
			It("returns an error", func() {
				_, err := Runner.Stats("invalid-id")
				Expect(err).To(MatchError(ContainSubstring("Image `invalid-id` not found. Skipping delete.")))
			})
		})

		Context("when the last parameter is a path", func() {
			It("returns an error", func() {
				invalidImagePath := filepath.Join(StorePath, store.ImageDirName, "not-here")
				_, err := Runner.Stats(invalidImagePath)
				Expect(err).To(MatchError(ContainSubstring("Image `not-here` not found. Skipping delete.")))
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
			_, err := Runner.Stats("")
			Expect(err).To(MatchError(ContainSubstring("invalid arguments")))
		})
	})
})
