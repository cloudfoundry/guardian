package groot_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	runnerpkg "code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Stats", func() {
	var (
		baseImagePath string
		image         groot.Image
	)

	BeforeEach(func() {
		var err error
		baseImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when image exists", func() {
		var expectedStats groot.VolumeStats

		BeforeEach(func() {
			cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(baseImagePath, "fatfile")), "bs=1048576", "count=5")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			image = integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id", 0)
			cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(image.RootFSPath, "hello")), "bs=1048576", "count=4")
			sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
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

		Describe("--config global flag", func() {
			var (
				configDir         string
				configFilePath    string
				configDraxBinPath string
			)

			BeforeEach(func() {
				var err error
				configDir, err = ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())
				configFilePath = path.Join(configDir, "config.yaml")
				configDraxBinPath = ""
			})

			JustBeforeEach(func() {
				cfg := config.Config{
					BaseStorePath: StorePath,
					DraxBin:       configDraxBinPath,
				}

				configYaml, err := yaml.Marshal(cfg)
				Expect(err).NotTo(HaveOccurred())

				Expect(ioutil.WriteFile(configFilePath, configYaml, 0755)).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.RemoveAll(configDir)).To(Succeed())
			})

			Describe("store path", func() {
				var (
					runner runnerpkg.Runner
				)

				BeforeEach(func() {
					runner = runnerpkg.Runner{
						GrootFSBin: GrootFSBin,
						DraxBin:    DraxBin,
					}.WithLogLevel(lager.DEBUG).WithStderr(GinkgoWriter).WithConfig(configFilePath)
				})

				It("uses the store path from the config file", func() {
					stats, err := runner.Stats("random-id")
					Expect(err).NotTo(HaveOccurred())

					Expect(stats).To(Equal(expectedStats))
				})
			})

			Describe("drax bin", func() {
				var (
					runner         runnerpkg.Runner
					draxCalledFile *os.File
					draxBin        *os.File
					tempFolder     string
				)

				BeforeEach(func() {
					tempFolder, draxBin, draxCalledFile = integration.CreateFakeDrax()
					configDraxBinPath = draxBin.Name()
				})

				JustBeforeEach(func() {
					runner = runnerpkg.Runner{
						GrootFSBin: GrootFSBin,
						StorePath:  StorePath,
					}.WithLogLevel(lager.DEBUG).WithStderr(GinkgoWriter).WithConfig(configFilePath)
				})

				It("uses the drax bin from the config file", func() {
					_, err := runner.Stats("random-id")
					Expect(err).To(MatchError("could not parse stats"))

					contents, err := ioutil.ReadFile(draxCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot - drax"))
				})
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
