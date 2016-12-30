package groot_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	runnerpkg "code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"
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

		Context("and ignored images flag is given", func() {
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

		Context("and a threshold is set", func() {
			var cleanupThresholdInBytes int64

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

	Context("when --config global flag is given and there are unused layers", func() {
		var (
			cfg              config.Config
			runnerWithConfig *runnerpkg.Runner
		)

		BeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:        "my-image-2",
				BaseImage: "docker:///busybox",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.Delete("my-image-2")).To(Succeed())

			cfg = config.Config{}
		})

		JustBeforeEach(func() {
			r := Runner
			runnerWithConfig = &r
			Expect(runnerWithConfig.SetConfig(cfg)).To(Succeed())
		})

		Describe("with a list of images to be ignored", func() {
			var preContents []os.FileInfo

			BeforeEach(func() {
				cfg.IgnoreBaseImages = []string{"docker:///busybox"}
			})

			JustBeforeEach(func() {
				var err error
				preContents, err = ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())
			})

			It("doesn't delete their layers", func() {
				_, err := runnerWithConfig.Clean(0, []string{})
				Expect(err).NotTo(HaveOccurred())

				afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())

				Expect(afterContents).To(Equal(preContents))
			})

			Context("when the ignore image flag is also given", func() {
				JustBeforeEach(func() {
					_, err := runnerWithConfig.Create(groot.CreateSpec{
						ID:        "my-image-3",
						BaseImage: "docker:///cfgarden/empty:v0.1.0",
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(runnerWithConfig.Delete("my-image-3")).To(Succeed())
				})

				It("does not ignore images provided in the config file", func() {
					_, err := runnerWithConfig.Clean(0, []string{"docker:///cfgarden/empty:v0.1.0"})
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())

					Expect(afterContents).NotTo(Equal(preContents))
				})
			})
		})

		Describe("clean up threshold", func() {
			Context("when threshold is not provided on the command line flag", func() {
				BeforeEach(func() {
					cfg.CleanThresholdBytes = 2500000
				})

				It("uses the threshold from the config file, and so does not clean", func() {
					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())

					_, err = runnerWithConfig.Clean(0, []string{})
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, CurrentUserID, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(len(preContents)))
				})
			})
		})
	})
})
