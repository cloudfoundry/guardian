package groot_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

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
	})

	Context("storage setup", func() {
		It("creates the storage path with the correct permission", func() {
			storePath := filepath.Join(StorePath, "new-store")
			Expect(storePath).ToNot(BeAnExistingFile())
			integration.CreateImage(GrootFSBin, storePath, DraxBin, baseImagePath, "random-id", tenMegabytes)
			Expect(storePath).To(BeADirectory())

			stat, err := os.Stat(storePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0700)))
		})

		Context("when fails to configure the store", func() {
			Describe("create", func() {
				It("logs the image id", func() {
					logBuffer := gbytes.NewBuffer()
					_, err := Runner.WithStore("/invalid-store").WithStderr(logBuffer).
						Create(groot.CreateSpec{
							ID:        "random-id",
							BaseImage: "my-image",
							DiskLimit: 12300,
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
			image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id", tenMegabytes)

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

						imagePath := path.Join(StorePath, "images", "random-id")
						Expect(imagePath).ToNot(BeAnExistingFile())
					})
				})
			})

			Context("when it's not provided", func() {
				It("uses drax from $PATH", func() {
					newPATH := fmt.Sprintf("%s:%s", tempFolder, os.Getenv("PATH"))
					cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", "--disk-limit-size-bytes", "104857600", baseImagePath, "random-id")
					cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", newPATH))
					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					contents, err := ioutil.ReadFile(draxCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot - drax"))
				})
			})
		})

		Describe("--btrfs-bin global flag", func() {
			var (
				btrfsCalledFile *os.File
				btrfsBin        *os.File
				tempFolder      string
			)

			BeforeEach(func() {
				tempFolder, btrfsBin, btrfsCalledFile = integration.CreateFakeBin("btrfs")
			})

			Context("when it's provided", func() {
				It("uses calls the provided btrfs binary", func() {
					_, err := Runner.WithBtrfsBin(btrfsBin.Name()).Create(groot.CreateSpec{BaseImage: baseImagePath, ID: "random-id"})
					Expect(err).To(HaveOccurred())

					contents, err := ioutil.ReadFile(btrfsCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot - btrfs"))
				})
			})

			Context("when it's not provided", func() {
				It("uses btrfs from $PATH", func() {
					newPATH := fmt.Sprintf("%s:%s", tempFolder, os.Getenv("PATH"))
					cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", baseImagePath, "random-id")
					cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", newPATH))
					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(1))

					contents, err := ioutil.ReadFile(btrfsCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot - btrfs"))
				})
			})
		})

	})

	Describe("uid and gid mappings", func() {
		Context("when creating two images based on the same base image", func() {
			var newuidmapBin *os.File

			JustBeforeEach(func() {
				_, newuidmapBin, _ = integration.CreateFakeBin("newuidmap")
				image, err := Runner.WithNewuidmapBin(newuidmapBin.Name()).Create(groot.CreateSpec{
					BaseImage:   baseImagePath,
					ID:          "foobar",
					UIDMappings: []groot.IDMappingSpec{groot.IDMappingSpec{HostID: 1000, NamespaceID: 1, Size: 1}},
					GIDMappings: []groot.IDMappingSpec{groot.IDMappingSpec{HostID: 1000, NamespaceID: 1, Size: 1}},
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

	Describe("--newuidmap-bin global flag", func() {
		var (
			newuidmapCalledFile *os.File
			newuidmapBin        *os.File
			tempFolder          string
		)

		BeforeEach(func() {
			tempFolder, newuidmapBin, newuidmapCalledFile = integration.CreateFakeBin("newuidmap")
		})

		Context("when it's provided", func() {
			It("uses calls the provided newuidmap binary", func() {
				_, err := Runner.WithNewuidmapBin(newuidmapBin.Name()).Create(groot.CreateSpec{
					BaseImage:   baseImagePath,
					ID:          "random-id",
					UIDMappings: []groot.IDMappingSpec{groot.IDMappingSpec{HostID: 1000, NamespaceID: 1, Size: 1}},
				})
				Expect(err).ToNot(HaveOccurred())

				contents, err := ioutil.ReadFile(newuidmapCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - newuidmap"))
			})
		})

		Context("when it's not provided", func() {
			It("uses newuidmap from $PATH", func() {
				newPATH := fmt.Sprintf("%s:%s", tempFolder, os.Getenv("PATH"))
				cmd := exec.Command(GrootFSBin, "--log-level", "debug", "--store", StorePath, "create", "--uid-mapping", "0:1000:1", baseImagePath, "random-id")
				cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", newPATH))
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				contents, err := ioutil.ReadFile(newuidmapCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - newuidmap"))
			})
		})
	})

	Describe("--newgidmap-bin global flag", func() {
		var (
			newgidmapCalledFile *os.File
			newgidmapBin        *os.File
			tempFolder          string
		)

		BeforeEach(func() {
			tempFolder, newgidmapBin, newgidmapCalledFile = integration.CreateFakeBin("newgidmap")
		})

		Context("when it's provided", func() {
			It("uses calls the provided newgidmap binary", func() {
				_, err := Runner.WithNewgidmapBin(newgidmapBin.Name()).Create(groot.CreateSpec{
					BaseImage:   baseImagePath,
					ID:          "random-id",
					GIDMappings: []groot.IDMappingSpec{groot.IDMappingSpec{HostID: 1000, NamespaceID: 1, Size: 1}},
				})
				Expect(err).ToNot(HaveOccurred())

				contents, err := ioutil.ReadFile(newgidmapCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - newgidmap"))
			})
		})

		Context("when it's not provided", func() {
			It("uses newgidmap from $PATH", func() {
				newPATH := fmt.Sprintf("%s:%s", tempFolder, os.Getenv("PATH"))
				cmd := exec.Command(GrootFSBin, "--log-level", "debug", "--store", StorePath, "create", "--gid-mapping", "0:1000:1", baseImagePath, "random-id")
				cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", newPATH))
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				contents, err := ioutil.ReadFile(newgidmapCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - newgidmap"))
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
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"--config",
				Runner.ConfigPath,
				"create",
				"--clean",
				"--no-clean",
				"docker:///cfgarden/empty:v0.1.1",
				"random-id",
			)
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, "10s").Should(gexec.Exit(1))
			Eventually(sess.Out).Should(gbytes.Say("clean and no-clean cannot be used together"))
		})
	})

	Context("when no --store option is given", func() {
		It("uses the default store path", func() {
			Expect("/var/lib/grootfs/images").ToNot(BeAnExistingFile())
			_, err := Runner.WithoutStore().Create(groot.CreateSpec{
				BaseImage: baseImagePath,
				ID:        "random-id",
			})
			Expect(err).To(MatchError(ContainSubstring(("making directory `/var/lib/grootfs`"))))
		})
	})

	Context("when two rootfses are using the same image", func() {
		It("isolates them", func() {
			image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id", 0)
			anotherImage := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "another-random-id", 0)
			Expect(ioutil.WriteFile(path.Join(image.RootFSPath, "bar"), []byte("hello-world"), 0644)).To(Succeed())
			Expect(path.Join(anotherImage.RootFSPath, "bar")).NotTo(BeARegularFile())
		})
	})

	Context("when the id is already being used", func() {
		JustBeforeEach(func() {
			Expect(integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id", 0)).NotTo(BeNil())
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

	Context("when groot does not have permissions to apply the requested mapping", func() {
		It("returns the newuidmap output in the stdout", func() {
			_, err := Runner.WithStore(StorePath).Create(groot.CreateSpec{
				BaseImage:   baseImagePath,
				UIDMappings: []groot.IDMappingSpec{groot.IDMappingSpec{HostID: 1, NamespaceID: 1, Size: 65000}},
				ID:          "some-id",
			})

			Expect(err).To(MatchError(MatchRegexp(`range [\[\)0-9\-]* -> [\[\)0-9\-]* not allowed`)))
		})

		It("does not leak the image directory", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"create",
				"--uid-mapping", "1:1:65000",
				baseImagePath,
				"some-id",
			)

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).NotTo(gexec.Exit(0))

			Expect(path.Join(StorePath, "images", "some-id")).ToNot(BeAnExistingFile())
		})
	})

	Context("when the image is invalid", func() {
		It("fails", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"create",
				"*@#%^!&",
				"some-id",
			)

			buffer := gbytes.NewBuffer()
			sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).To(gexec.Exit(1))
			Eventually(sess).Should(gbytes.Say("parsing image url: parse"))
			Eventually(sess).Should(gbytes.Say("invalid URL escape"))
		})
	})

	Context("when a mappings flag is invalid", func() {
		It("fails when the uid mapping is invalid", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"create", baseImagePath,
				"--uid-mapping", "1:hello:65000",
				"some-id",
			)

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).NotTo(gexec.Exit(0))
		})

		It("fails when the gid mapping is invalid", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"create", baseImagePath,
				"--gid-mapping", "1:groot:65000",
				"some-id",
			)

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).NotTo(gexec.Exit(0))
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

		Describe("btrfs bin", func() {
			var (
				btrfsCalledFile *os.File
				btrfsBin        *os.File
				tempFolder      string
			)

			BeforeEach(func() {
				tempFolder, btrfsBin, btrfsCalledFile = integration.CreateFakeBin("btrfs")
				cfg.BtrfsBin = btrfsBin.Name()
			})

			It("uses the btrfs bin from the config file", func() {
				_, err := Runner.Create(groot.CreateSpec{
					BaseImage: baseImagePath,
					ID:        "random-id",
				})
				Expect(err).To(HaveOccurred())

				contents, err := ioutil.ReadFile(btrfsCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - btrfs"))
			})
		})

		Describe("newuidmap bin", func() {
			var (
				newuidmapCalledFile *os.File
				newuidmapBin        *os.File
				tempFolder          string
			)

			BeforeEach(func() {
				tempFolder, newuidmapBin, newuidmapCalledFile = integration.CreateFakeBin("newuidmap")
				cfg.NewuidmapBin = newuidmapBin.Name()
			})

			It("uses the newuidmap bin from the config file", func() {
				_, err := Runner.Create(groot.CreateSpec{
					BaseImage:   baseImagePath,
					UIDMappings: []groot.IDMappingSpec{groot.IDMappingSpec{HostID: 1000, NamespaceID: 1, Size: 1}},
					ID:          "random-id",
				})
				Expect(err).ToNot(HaveOccurred())

				contents, err := ioutil.ReadFile(newuidmapCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - newuidmap"))
			})
		})

		Describe("newgidmap bin", func() {
			var (
				newgidmapCalledFile *os.File
				newgidmapBin        *os.File
				tempFolder          string
			)

			BeforeEach(func() {
				tempFolder, newgidmapBin, newgidmapCalledFile = integration.CreateFakeBin("newgidmap")
				cfg.NewgidmapBin = newgidmapBin.Name()
			})

			It("uses the newgidmap bin from the config file", func() {
				_, err := Runner.Create(groot.CreateSpec{
					BaseImage:   baseImagePath,
					GIDMappings: []groot.IDMappingSpec{groot.IDMappingSpec{HostID: 1000, NamespaceID: 1, Size: 1}},
					ID:          "random-id",
				})
				Expect(err).ToNot(HaveOccurred())

				contents, err := ioutil.ReadFile(newgidmapCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - newgidmap"))
			})
		})

		Describe("uid mappings", func() {
			BeforeEach(func() {
				cfg.UIDMappings = []string{"1:1:65990"}
			})

			It("uses the uid mappings from the config file", func() {
				buffer := gbytes.NewBuffer()
				_, err := Runner.WithStdout(buffer).Create(spec)
				Expect(err).To(HaveOccurred())
				Expect(buffer.Contents()).To(ContainSubstring("uid range [1-65991) -> [1-65991) not allowed"))
			})
		})

		Describe("gid mappings", func() {
			BeforeEach(func() {
				cfg.GIDMappings = []string{"1:1:65990"}
			})

			It("uses the gid mappings from the config file", func() {
				buffer := gbytes.NewBuffer()
				_, err := Runner.WithStdout(buffer).Create(spec)
				Expect(err).To(HaveOccurred())
				Expect(string(buffer.Contents())).To(ContainSubstring("gid range [1-65991) -> [1-65991) not allowed"))
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

		Describe("clean up on create", func() {
			var createSpec groot.CreateSpec

			BeforeEach(func() {
				cfg.CleanOnCreate = true
			})

			JustBeforeEach(func() {
				createSpec = groot.CreateSpec{
					ID:        "my-busybox",
					BaseImage: "docker:///busybox:1.26.2",
				}

				_, err := Runner.Create(createSpec)
				Expect(err).NotTo(HaveOccurred())

				Expect(Runner.Delete("my-busybox")).To(Succeed())
			})

			AfterEach(func() {
				Expect(Runner.Delete(spec.ID)).To(Succeed())
			})

			It("cleans up unused layers before create but not the one about to be created", func() {
				createSpec = groot.CreateSpec{
					ID:        "my-empty",
					BaseImage: "docker:///cfgarden/empty:v0.1.1",
				}
				_, err := Runner.Create(createSpec)
				Expect(err).NotTo(HaveOccurred())
				Expect(Runner.Delete("my-empty")).To(Succeed())

				layerPath := filepath.Join(StorePath, store.VOLUMES_DIR_NAME, testhelpers.EmptyBaseImageV011.Layers[0].ChainID)
				stat, err := os.Stat(layerPath)
				Expect(err).NotTo(HaveOccurred())
				preLayerTimestamp := stat.ModTime()

				preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VOLUMES_DIR_NAME))
				Expect(err).NotTo(HaveOccurred())
				Expect(preContents).To(HaveLen(3))

				spec.BaseImage = "docker:///cfgarden/empty:v0.1.1"
				cmd := exec.Command(
					GrootFSBin, "--store", StorePath,
					"--config",
					Runner.ConfigPath,
					"create",
					"docker:///cfgarden/empty:v0.1.1",
					spec.ID,
				)
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess, "15s").Should(gexec.Exit(0))

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

					cmd := exec.Command(
						GrootFSBin, "--store", StorePath,
						"--config",
						Runner.ConfigPath,
						"create",
						"--no-clean",
						"docker:///cfgarden/empty:v0.1.1",
						spec.ID,
					)
					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess, "10s").Should(gexec.Exit(0))

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VOLUMES_DIR_NAME))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(3))
				})
			})
		})
	})
})
