package groot_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

const (
	tenMegabytes = int64(10485760)
)

var _ = Describe("Create", func() {
	var (
		baseImagePath   string
		sourceImagePath string
	)

	BeforeEach(func() {
		integration.SkipIfNotBTRFS(Driver)

		var err error
		sourceImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "root-owned"), []byte{}, 0644)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
	})

	Context("storage setup", func() {
		It("creates the storage path with the correct permission", func() {
			storePath := filepath.Join(StorePath, "new-store")
			Expect(storePath).ToNot(BeAnExistingFile())
			_, err := Runner.WithStore(storePath).Create(groot.CreateSpec{
				BaseImage: baseImagePath,
				ID:        "random-id",
				DiskLimit: tenMegabytes,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(storePath).To(BeADirectory())
			stat, err := os.Stat(storePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0700)))
		})

		Context("when fails to configure the store", func() {
			Describe("create", func() {
				It("logs the image id", func() {
					logBuffer := gbytes.NewBuffer()
					_, err := Runner.WithStore("/invalid/store/path").WithStderr(logBuffer).
						Create(groot.CreateSpec{
							ID:        "random-id",
							BaseImage: "my-image",
						})
					Expect(err).To(HaveOccurred())
					Expect(logBuffer).To(gbytes.Say(`"id":"random-id"`))
				})
			})
		})
	})

	Context("when inclusive disk limit is provided", func() {
		BeforeEach(func() {
			Expect(writeMegabytes(filepath.Join(sourceImagePath, "fatfile"), 5)).To(Succeed())
		})

		It("creates a image with supplied limit", func() {
			image, err := Runner.Create(groot.CreateSpec{
				BaseImage: baseImagePath,
				ID:        "random-id",
				DiskLimit: tenMegabytes,
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(writeMegabytes(filepath.Join(image.RootFSPath, "hello"), 4)).To(Succeed())
			Expect(writeMegabytes(filepath.Join(image.RootFSPath, "hello2"), 2)).To(MatchError(ContainSubstring("Disk quota exceeded")))
		})

		Context("when the disk limit value is invalid", func() {
			It("fails with a helpful error", func() {
				_, err := Runner.Create(groot.CreateSpec{
					DiskLimit: -200,
					BaseImage: baseImagePath,
					ID:        "random-id",
				})
				Expect(err).To(MatchError(ContainSubstring("disk limit cannot be negative")))
			})
		})

		Context("when the exclude-image-from-quota is also provided", func() {
			It("creates a image with supplied limit, but doesn't take into account the base image size", func() {
				image, err := Runner.Create(groot.CreateSpec{
					DiskLimit:                 10485760,
					ExcludeBaseImageFromQuota: true,
					BaseImage:                 baseImagePath,
					ID:                        "random-id",
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(writeMegabytes(filepath.Join(image.RootFSPath, "hello"), 6)).To(Succeed())
				Expect(writeMegabytes(filepath.Join(image.RootFSPath, "hello2"), 5)).To(MatchError(ContainSubstring("Disk quota exceeded")))
			})
		})

		Describe("--drax-bin global flag", func() {
			var (
				draxCalledFile *os.File
				draxBin        *os.File
				tempFolder     string
			)

			BeforeEach(func() {
				tempFolder, draxBin, draxCalledFile = integration.CreateFakeDrax()
			})

			Context("when it's provided", func() {
				It("uses the provided drax", func() {
					_, err := Runner.WithDraxBin(draxBin.Name()).Create(groot.CreateSpec{
						BaseImage: baseImagePath,
						ID:        "random-id",
						DiskLimit: 104857600,
					})
					Expect(err).NotTo(HaveOccurred())

					contents, err := ioutil.ReadFile(draxCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot - drax"))
				})

				Context("when the drax bin doesn't have uid bit set", func() {
					It("doesn't leak the image dir", func() {
						testhelpers.UnsuidDrax(draxBin.Name())
						_, err := Runner.WithDraxBin(draxBin.Name()).Create(groot.CreateSpec{
							BaseImage: baseImagePath,
							ID:        "random-id",
							DiskLimit: 104857600,
						})
						Expect(err).To(HaveOccurred())

						imagePath := path.Join(Runner.StorePath, "images", "random-id")
						Expect(imagePath).ToNot(BeAnExistingFile())
					})
				})
			})

			Context("when it's not provided", func() {
				It("uses drax from $PATH", func() {
					newPATH := fmt.Sprintf("%s:%s", tempFolder, os.Getenv("PATH"))
					_, err := Runner.WithoutDraxBin().WithEnvVar(fmt.Sprintf("PATH=%s", newPATH)).Create(groot.CreateSpec{
						BaseImage: baseImagePath,
						ID:        "random-id",
						DiskLimit: tenMegabytes,
					})
					Expect(err).ToNot(HaveOccurred())

					contents, err := ioutil.ReadFile(draxCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot - drax"))
				})
			})
		})
	})

	Describe("unique uid and gid mappings per store", func() {
		Context("when creating two images with different mappings", func() {
			var newuidmapBin *os.File

			JustBeforeEach(func() {
				_, newuidmapBin, _ = integration.CreateFakeBin("newuidmap")
				image, err := Runner.WithNewuidmapBin(newuidmapBin.Name()).Create(groot.CreateSpec{
					BaseImage:   baseImagePath,
					ID:          "foobar",
					UIDMappings: []groot.IDMappingSpec{groot.IDMappingSpec{HostID: int(GrootUID), NamespaceID: 0, Size: 1}},
					GIDMappings: []groot.IDMappingSpec{groot.IDMappingSpec{HostID: int(GrootGID), NamespaceID: 0, Size: 1}},
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(image.Path).To(BeADirectory())
			})

			It("returns a useful error message", func() {
				_, err := Runner.WithNewuidmapBin(newuidmapBin.Name()).Create(groot.CreateSpec{
					BaseImage: baseImagePath,
					ID:        "foobar2",
				})
				Expect(err).To(MatchError("store already initialized with a different mapping"))
			})
		})
	})

	Context("when --clean is given", func() {
		BeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:        "my-busybox",
				BaseImage: "docker:///busybox:1.26.2",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(Runner.Delete("my-busybox")).To(Succeed())
		})

		AfterEach(func() {
			Runner.Delete("my-empty")
		})

		It("cleans the store first", func() {
			preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VOLUMES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(preContents).To(HaveLen(1))

			_, err = Runner.Create(groot.CreateSpec{
				ID:                          "my-empty",
				BaseImage:                   "docker:///cfgarden/empty:v0.1.1",
				CleanOnCreate:               true,
				CleanOnCreateIgnoreImages:   []string{"docker://my-image"},
				CleanOnCreateThresholdBytes: int64(250000),
			})
			Expect(err).NotTo(HaveOccurred())

			afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VOLUMES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(afterContents).To(HaveLen(2))
			for _, layer := range testhelpers.EmptyBaseImageV011.Layers {
				Expect(filepath.Join(StorePath, store.VOLUMES_DIR_NAME, layer.ChainID)).To(BeADirectory())
			}
		})
	})

	Context("when --no-clean is given", func() {
		BeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:        "my-busybox",
				BaseImage: "docker:///busybox:1.26.2",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(Runner.Delete("my-busybox")).To(Succeed())
		})

		AfterEach(func() {
			Runner.Delete("my-empty")
		})

		It("does not clean the store", func() {
			preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VOLUMES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(preContents).To(HaveLen(1))

			_, err = Runner.Create(groot.CreateSpec{
				ID:                          "my-empty",
				BaseImage:                   "docker:///cfgarden/empty:v0.1.1",
				CleanOnCreate:               false,
				CleanOnCreateIgnoreImages:   []string{"docker://my-image"},
				CleanOnCreateThresholdBytes: int64(250000),
			})
			Expect(err).NotTo(HaveOccurred())

			afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VOLUMES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(afterContents).To(HaveLen(3))

			layers := append(testhelpers.EmptyBaseImageV011.Layers, testhelpers.BusyBoxImage.Layers...)
			for _, layer := range layers {
				Expect(filepath.Join(StorePath, store.VOLUMES_DIR_NAME, layer.ChainID)).To(BeADirectory())
			}
		})
	})

	Context("when both no-clean and clean flags are given", func() {
		It("returns an error", func() {
			_, err := Runner.WithClean().WithNoClean().Create(groot.CreateSpec{
				ID:        "my-empty",
				BaseImage: "docker:///cfgarden/empty:v0.1.1",
			})
			Expect(err).To(MatchError(ContainSubstring("clean and no-clean cannot be used together")))
		})
	})

	Context("when no --store option is given", func() {
		It("uses the default store path", func() {
			Expect("/var/lib/grootfs/images").ToNot(BeAnExistingFile())
			_, err := Runner.WithoutStore().Create(groot.CreateSpec{
				BaseImage: baseImagePath,
				ID:        "random-id",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect("/var/lib/grootfs/images").To(BeADirectory())
		})
	})

	Context("when two rootfses are using the same image", func() {
		It("isolates them", func() {
			image1, err := Runner.Create(groot.CreateSpec{
				ID:        "random-id",
				BaseImage: baseImagePath,
			})
			Expect(err).NotTo(HaveOccurred())

			image2, err := Runner.Create(groot.CreateSpec{
				ID:        "another-random-id",
				BaseImage: baseImagePath,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(filepath.Join(image1.RootFSPath, "bar"), []byte("hello-world"), 0644)).To(Succeed())
			Expect(filepath.Join(image2.RootFSPath, "bar")).NotTo(BeARegularFile())
		})
	})

	Context("when the id is already being used", func() {
		JustBeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:        "random-id",
				BaseImage: baseImagePath,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("fails and produces a useful error", func() {
			_, err := Runner.WithStore(StorePath).Create(groot.CreateSpec{
				BaseImage: baseImagePath,
				ID:        "random-id",
			})
			Expect(err).To(MatchError(ContainSubstring("image for id `random-id` already exists")))
		})
	})

	Context("when the id is not provided", func() {
		It("fails", func() {
			_, err := Runner.WithStore(StorePath).Create(groot.CreateSpec{
				BaseImage: baseImagePath,
				ID:        "",
			})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the id contains invalid characters", func() {
		It("fails", func() {
			_, err := Runner.WithStore(StorePath).Create(groot.CreateSpec{
				BaseImage: baseImagePath,
				ID:        "this/is/not/okay",
			})
			Expect(err).To(MatchError(ContainSubstring("id `this/is/not/okay` contains invalid characters: `/`")))
		})
	})

	Context("when the image is invalid", func() {
		It("fails", func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:        "some-id",
				BaseImage: "*@#%^!&",
			})
			Expect(err).To(MatchError(ContainSubstring("parsing image url: parse")))
			Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
		})
	})

	Context("when a mappings flag is invalid", func() {
		It("fails when the uid mapping is invalid", func() {
			cmd := exec.Command(
				GrootFSBin,
				"--store", StorePath,
				"--driver", Driver,
				"create", baseImagePath,
				"--uid-mapping", "1:hello:65000",
				"some-id",
			)

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).NotTo(gexec.Exit(0))
			Eventually(sess).Should(gbytes.Say("parsing uid-mapping: expected integer"))
		})

		It("fails when the gid mapping is invalid", func() {
			cmd := exec.Command(
				GrootFSBin,
				"--store", StorePath,
				"--driver", Driver,
				"create", baseImagePath,
				"--gid-mapping", "1:groot:65000",
				"some-id",
			)

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).NotTo(gexec.Exit(0))
			Eventually(sess).Should(gbytes.Say("parsing gid-mapping: expected integer"))
		})
	})

	Describe("--config global flag", func() {
		var (
			cfg  config.Config
			spec groot.CreateSpec
		)

		BeforeEach(func() {
			cfg = config.Config{}
		})

		JustBeforeEach(func() {
			spec = groot.CreateSpec{
				ID:        "random-id",
				BaseImage: baseImagePath,
			}
		})

		JustBeforeEach(func() {
			Expect(Runner.SetConfig(cfg)).To(Succeed())
		})

		Describe("store path", func() {
			BeforeEach(func() {
				var err error
				cfg.StorePath, err = ioutil.TempDir(StorePath, "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("uses the store path from the config file", func() {
				image, err := Runner.WithoutStore().Create(spec)
				Expect(err).NotTo(HaveOccurred())
				Expect(image.Path).To(Equal(filepath.Join(cfg.StorePath, "images/random-id")))
			})
		})

		Describe("drax bin", func() {
			var (
				draxCalledFile *os.File
				draxBin        *os.File
				tempFolder     string
			)

			BeforeEach(func() {
				tempFolder, draxBin, draxCalledFile = integration.CreateFakeDrax()
				cfg.DraxBin = draxBin.Name()
			})

			It("uses the drax bin from the config file", func() {
				_, err := Runner.WithoutDraxBin().Create(groot.CreateSpec{
					BaseImage: baseImagePath,
					ID:        "random-id",
					DiskLimit: 104857600,
				})
				Expect(err).NotTo(HaveOccurred())

				contents, err := ioutil.ReadFile(draxCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - drax"))
			})
		})

		Describe("mappings", func() {
			BeforeEach(func() {
				cfg.UIDMappings = []string{"1:1001:65990", "0:500:1"}
				cfg.GIDMappings = []string{"1:1001:65990", "0:501:1"}
			})

			It("uses the uid mappings from the config file", func() {
				image, err := Runner.Create(spec)
				Expect(err).NotTo(HaveOccurred())

				rootOwnedFile, err := os.Stat(filepath.Join(image.RootFSPath, "root-owned"))
				Expect(err).NotTo(HaveOccurred())
				Expect(rootOwnedFile.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(500)))
				Expect(rootOwnedFile.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(501)))
			})
		})

		Describe("disk limit size bytes", func() {
			BeforeEach(func() {
				cfg.DiskLimitSizeBytes = tenMegabytes
			})

			It("creates a image with limit from the config file", func() {
				image, err := Runner.Create(spec)
				Expect(err).ToNot(HaveOccurred())

				Expect(writeMegabytes(filepath.Join(image.RootFSPath, "hello"), 11)).To(MatchError(ContainSubstring("Disk quota exceeded")))
			})
		})

		Describe("exclude image from quota", func() {
			BeforeEach(func() {
				cfg.ExcludeBaseImageFromQuota = true
				cfg.DiskLimitSizeBytes = tenMegabytes
			})

			It("excludes base image from quota when config property say so", func() {
				image, err := Runner.Create(spec)
				Expect(err).ToNot(HaveOccurred())

				Expect(writeMegabytes(filepath.Join(image.RootFSPath, "hello"), 6)).To(Succeed())
				Expect(writeMegabytes(filepath.Join(image.RootFSPath, "hello2"), 5)).To(MatchError(ContainSubstring("Disk quota exceeded")))
			})
		})
	})

	Describe("clean up on create", func() {
		var (
			imageID string
		)

		JustBeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:        "my-busybox",
				BaseImage: "docker:///busybox:1.26.2",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(Runner.Delete("my-busybox")).To(Succeed())
			imageID = "random-id"
		})

		AfterEach(func() {
			Expect(Runner.Delete(imageID)).To(Succeed())
		})

		It("cleans up unused layers before create but not the one about to be created", func() {
			runner := Runner.WithClean()

			createSpec := groot.CreateSpec{
				ID:        "my-empty",
				BaseImage: "docker:///cfgarden/empty:v0.1.1",
			}
			_, err := Runner.Create(createSpec)
			Expect(err).NotTo(HaveOccurred())
			Expect(runner.Delete("my-empty")).To(Succeed())

			layerPath := filepath.Join(StorePath, store.VOLUMES_DIR_NAME, testhelpers.EmptyBaseImageV011.Layers[0].ChainID)
			stat, err := os.Stat(layerPath)
			Expect(err).NotTo(HaveOccurred())
			preLayerTimestamp := stat.ModTime()

			preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VOLUMES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(preContents).To(HaveLen(3))

			_, err = runner.Create(groot.CreateSpec{
				ID:        imageID,
				BaseImage: "docker:///cfgarden/empty:v0.1.1",
			})
			Expect(err).NotTo(HaveOccurred())

			afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VOLUMES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(afterContents).To(HaveLen(2))

			for _, layer := range testhelpers.EmptyBaseImageV011.Layers {
				Expect(filepath.Join(StorePath, store.VOLUMES_DIR_NAME, layer.ChainID)).To(BeADirectory())
			}

			stat, err = os.Stat(layerPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.ModTime()).To(Equal(preLayerTimestamp))
		})

		Context("when no-clean flag is set", func() {
			It("does not clean up unused layers", func() {
				preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VOLUMES_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())
				Expect(preContents).To(HaveLen(1))

				_, err = Runner.WithNoClean().Create(groot.CreateSpec{
					ID:        imageID,
					BaseImage: "docker:///cfgarden/empty:v0.1.1",
				})
				Expect(err).NotTo(HaveOccurred())

				afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VOLUMES_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())
				Expect(afterContents).To(HaveLen(3))
			})
		})
	})
})
