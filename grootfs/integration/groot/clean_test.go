package groot_test

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	runnerpkg "code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Clean", func() {
	BeforeEach(func() {
		_, err := Runner.Create(groot.CreateSpec{
			ID:        "my-image-1",
			BaseImage: "docker:///cfgarden/empty:v0.1.1",
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(Runner.Delete("my-image-1")).To(Succeed())
	})

	It("removes the cached blobs", func() {
		preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.CACHE_DIR_NAME))
		Expect(err).NotTo(HaveOccurred())
		Expect(len(preContents)).To(BeNumerically(">", 0))

		_, err = Runner.Clean(0, []string{})
		Expect(err).NotTo(HaveOccurred())

		afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.CACHE_DIR_NAME))
		Expect(err).NotTo(HaveOccurred())
		Expect(afterContents).To(HaveLen(0))
	})

	Context("when there are unused layers", func() {
		BeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:        "my-image-2",
				BaseImage: "docker:///busybox",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(Runner.Delete("my-image-2")).To(Succeed())
		})

		It("removes unused volumes", func() {
			preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(preContents).To(HaveLen(3))

			_, err = Runner.Clean(0, []string{})
			Expect(err).NotTo(HaveOccurred())

			afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(afterContents).To(HaveLen(2))
			for _, layer := range testhelpers.EmptyBaseImageV011.Layers {
				Expect(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME, layer.ChainID)).To(BeADirectory())
			}
		})

		Context("when ignored images flag is given", func() {
			var preContents []os.FileInfo

			JustBeforeEach(func() {
				var err error
				preContents, err = ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())
			})

			It("doesn't delete their layers", func() {
				_, err := Runner.Clean(0, []string{"docker:///busybox"})
				Expect(err).NotTo(HaveOccurred())

				afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())

				Expect(afterContents).To(Equal(preContents))
			})

			Context("when more than one image is to be ignored", func() {
				BeforeEach(func() {
					_, err := Runner.Create(groot.CreateSpec{
						ID:        "my-image-3",
						BaseImage: "docker:///cfgarden/empty:v0.1.0",
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(Runner.Delete("my-image-3")).To(Succeed())
				})

				It("doesn't delete their layers", func() {
					_, err := Runner.Clean(0, []string{"docker:///busybox", "docker:///cfgarden/empty:v0.1.0"})
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())

					Expect(afterContents).To(Equal(preContents))
				})
			})
		})

		Context("when --config global flag is given", func() {
			var (
				configStorePath   string
				configDir         string
				configFilePath    string
				configDraxBinPath string
				ignoredImagesList []string
			)

			BeforeEach(func() {
				var err error
				configDir, err = ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())
				configFilePath = path.Join(configDir, "config.yaml")
				configStorePath = StorePath
				ignoredImagesList = []string{}
				configDraxBinPath = ""
			})

			JustBeforeEach(func() {
				cfg := config.Config{
					BaseStorePath:    configStorePath,
					IgnoreBaseImages: ignoredImagesList,
					DraxBin:          configDraxBinPath,
				}

				configYaml, err := yaml.Marshal(cfg)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.WriteFile(configFilePath, configYaml, 0755)).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.RemoveAll(configDir)).To(Succeed())
			})

			Describe("with a list of images to be ignored", func() {
				var preContents []os.FileInfo

				BeforeEach(func() {
					ignoredImagesList = []string{"docker:///busybox"}
				})

				JustBeforeEach(func() {
					var err error
					preContents, err = ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
				})

				It("doesn't delete their layers", func() {
					_, err := Runner.WithConfig(configFilePath).Clean(0, []string{})
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())

					Expect(afterContents).To(Equal(preContents))
				})

				Context("when the ignore image flag is also given", func() {
					BeforeEach(func() {
						_, err := Runner.Create(groot.CreateSpec{
							ID:        "my-image-3",
							BaseImage: "docker:///cfgarden/empty:v0.1.0",
						})
						Expect(err).NotTo(HaveOccurred())

						Expect(Runner.Delete("my-image-3")).To(Succeed())
					})

					It("does not ignore images provided in the config file", func() {
						_, err := Runner.WithConfig(configFilePath).Clean(0, []string{"docker:///cfgarden/empty:v0.1.0"})
						Expect(err).NotTo(HaveOccurred())

						afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
						Expect(err).NotTo(HaveOccurred())

						Expect(afterContents).NotTo(Equal(preContents))
					})
				})
			})

			Describe("store path", func() {
				var runner runnerpkg.Runner

				BeforeEach(func() {
					runner = runnerpkg.Runner{
						GrootFSBin: GrootFSBin,
						DraxBin:    DraxBin,
					}.WithLogLevel(lager.DEBUG).WithStderr(GinkgoWriter).WithConfig(configFilePath)

					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(preContents).To(HaveLen(3))
				})

				It("uses the store path from the config file", func() {
					_, err := runner.Clean(0, []string{})
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(2))
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
					_, err := runner.Clean(0, []string{})
					Expect(err).NotTo(HaveOccurred())

					contents, err := ioutil.ReadFile(draxCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot"))
				})
			})
		})

		Context("and a threshold is set", func() {
			var cleanupThresholdInBytes uint64

			Context("and the total is less than the threshold", func() {
				BeforeEach(func() {
					// 688128      # Blob cache
					// 16384       # Empty layer
					// 16384       # Empty layer
					// 16384       # Empty rootfs
					// 1179648     # Busybox layer
					// ----------------------------------
					// = 1916928 ~= 1.83 MB

					cleanupThresholdInBytes = 2500000
				})

				It("does not remove the cached blobs", func() {
					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.CACHE_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())

					_, err = Runner.Clean(cleanupThresholdInBytes, []string{})
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.CACHE_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(len(preContents)))
				})

				It("does not remove the unused volumes", func() {
					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())

					_, err = Runner.Clean(cleanupThresholdInBytes, []string{})
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(len(preContents)))
				})

				It("reports that it was a no-op", func() {
					output, err := Runner.Clean(cleanupThresholdInBytes, []string{})
					Expect(err).NotTo(HaveOccurred())
					Expect(output).To(ContainSubstring("threshold not reached: skipping clean"))
				})
			})

			Context("and the total is more than the threshold", func() {
				BeforeEach(func() {
					cleanupThresholdInBytes = 70000
				})

				It("removes the cached blobs", func() {
					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.CACHE_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(preContents).To(HaveLen(2))

					_, err = Runner.Clean(cleanupThresholdInBytes, []string{})
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.CACHE_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(0))
				})

				It("removes the unused volumes", func() {
					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(preContents).To(HaveLen(3))

					_, err = Runner.Clean(cleanupThresholdInBytes, []string{})
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(2))
					for _, layer := range testhelpers.EmptyBaseImageV011.Layers {
						Expect(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME, layer.ChainID)).To(BeADirectory())
					}
				})
			})
		})
	})
})
