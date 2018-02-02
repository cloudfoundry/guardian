package integration_test

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
	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	tenMegabytes = int64(10485760)
	rootUID      = 0
	rootGID      = 0
)

var _ = Describe("Create", func() {
	var (
		randomImageID   string
		baseImagePath   string
		sourceImagePath string
	)

	BeforeEach(func() {
		randomImageID = testhelpers.NewRandomID()
		sourceImagePath = integration.CreateBaseImage(rootUID, rootGID, GrootUID, GrootGID)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
	})

	It("keeps the ownership and permissions", func() {
		integration.SkipIfNonRoot(GrootfsTestUid)

		containerSpec, err := Runner.Create(groot.CreateSpec{
			BaseImageURL: integration.String2URL(baseImagePath),
			ID:           randomImageID,
			Mount:        mountByDefault(),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())
		grootFi, err := os.Stat(path.Join(containerSpec.Root.Path, "foo"))
		Expect(err).NotTo(HaveOccurred())
		Expect(grootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
		Expect(grootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))

		grootLink, err := os.Lstat(path.Join(containerSpec.Root.Path, "groot-link"))
		Expect(err).NotTo(HaveOccurred())
		Expect(grootLink.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
		Expect(grootLink.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))

		rootFi, err := os.Stat(path.Join(containerSpec.Root.Path, "bar"))
		Expect(err).NotTo(HaveOccurred())
		Expect(rootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(rootUID)))
		Expect(rootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(rootGID)))
	})

	Context("when the store isn't initialized", func() {
		var runner runner.Runner

		BeforeEach(func() {
			runner = Runner.SkipInitStore()
		})

		It("returns an error", func() {
			_, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(baseImagePath),
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).To(MatchError(ContainSubstring("Store path is not initialized. Please run init-store.")))
		})
	})

	Context("when mappings are applied on store initialization", func() {
		BeforeEach(func() {
			initSpec := runner.InitSpec{
				UIDMappings: []groot.IDMappingSpec{
					{HostID: GrootUID, NamespaceID: 0, Size: 1},
					{HostID: 100000, NamespaceID: 1, Size: 65000},
				},
				GIDMappings: []groot.IDMappingSpec{
					{HostID: GrootGID, NamespaceID: 0, Size: 1},
					{HostID: 100000, NamespaceID: 1, Size: 65000},
				},
			}

			Expect(Runner.RunningAsUser(0, 0).InitStore(initSpec)).To(Succeed())
		})

		It("applies the same mappings to the created image", func() {
			containerSpec, err := Runner.WithLogLevel(lager.DEBUG).SkipInitStore().
				Create(groot.CreateSpec{
					ID:           "some-id",
					BaseImageURL: integration.String2URL(baseImagePath),
					Mount:        mountByDefault(),
				})
			Expect(err).NotTo(HaveOccurred())

			Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())

			grootFile, err := os.Stat(path.Join(containerSpec.Root.Path, "foo"))
			Expect(err).NotTo(HaveOccurred())
			Expect(grootFile.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID + 99999)))
			Expect(grootFile.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID + 99999)))

			grootDir, err := os.Stat(path.Join(containerSpec.Root.Path, "groot-folder"))
			Expect(err).NotTo(HaveOccurred())
			Expect(grootDir.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID + 99999)))
			Expect(grootDir.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID + 99999)))

			grootLink, err := os.Lstat(path.Join(containerSpec.Root.Path, "groot-link"))
			Expect(err).NotTo(HaveOccurred())
			Expect(grootLink.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID + 99999)))
			Expect(grootLink.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID + 99999)))

			rootFile, err := os.Stat(path.Join(containerSpec.Root.Path, "bar"))
			Expect(err).NotTo(HaveOccurred())
			Expect(rootFile.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
			Expect(rootFile.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))

			rootDir, err := os.Stat(path.Join(containerSpec.Root.Path, "root-folder"))
			Expect(err).NotTo(HaveOccurred())
			Expect(rootDir.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
			Expect(rootDir.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))
		})

		It("allows the mapped user to have access to the created image", func() {
			containerSpec, err := Runner.WithLogLevel(lager.DEBUG).SkipInitStore().
				Create(groot.CreateSpec{
					Mount:        mountByDefault(),
					ID:           "some-id",
					BaseImageURL: integration.String2URL(baseImagePath),
				})
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())

			listRootfsCmd := exec.Command("ls", filepath.Join(containerSpec.Root.Path, "root-folder"))
			listRootfsCmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: uint32(GrootUID),
					Gid: uint32(GrootGID),
				},
			}

			sess, err := gexec.Start(listRootfsCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})
	})

	Context("when disk limit is provided", func() {
		BeforeEach(func() {
			Expect(writeMegabytes(filepath.Join(sourceImagePath, "fatfile"), 5)).To(Succeed())
		})

		It("creates an image with supplied limit", func() {
			containerSpec, err := Runner.WithLogLevel(lager.DEBUG).Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(baseImagePath),
				ID:           randomImageID,
				DiskLimit:    tenMegabytes,
				Mount:        mountByDefault(),
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(writeMegabytes(filepath.Join(containerSpec.Root.Path, "hello"), 4)).To(Succeed())
			Expect(writeMegabytes(filepath.Join(containerSpec.Root.Path, "hello2"), 2)).To(MatchError(ContainSubstring("dd: error writing")))
		})

		Context("when the disk limit value is invalid", func() {
			It("fails with a helpful error", func() {
				_, err := Runner.Create(groot.CreateSpec{
					DiskLimit:    -200,
					BaseImageURL: integration.String2URL(baseImagePath),
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).To(MatchError(ContainSubstring("disk limit cannot be negative")))
			})
		})

		Context("when the exclude-image-from-quota is also provided", func() {
			It("creates a image with supplied limit, but doesn't take into account the base image size", func() {
				containerSpec, err := Runner.Create(groot.CreateSpec{
					DiskLimit:                 10485760,
					ExcludeBaseImageFromQuota: true,
					BaseImageURL:              integration.String2URL(baseImagePath),
					ID:                        randomImageID,
					Mount:                     mountByDefault(),
				})
				Expect(err).ToNot(HaveOccurred())
				_, err = Runner.Stats(randomImageID)
				Expect(err).NotTo(HaveOccurred())

				Expect(writeMegabytes(filepath.Join(containerSpec.Root.Path, "hello"), 6)).To(Succeed())
				Expect(writeMegabytes(filepath.Join(containerSpec.Root.Path, "hello2"), 5)).To(MatchError(ContainSubstring("dd: error writing")))
			})
		})
	})

	Context("when --with-clean is given", func() {
		BeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:           "my-busybox",
				BaseImageURL: integration.String2URL("docker:///cfgarden/garden-busybox"),
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.Delete("my-busybox")).To(Succeed())
		})

		AfterEach(func() {
			_ = Runner.Delete("my-empty")
		})

		It("cleans the store first", func() {
			preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
			Expect(err).NotTo(HaveOccurred())
			Expect(preContents).To(HaveLen(1))

			_, err = Runner.Create(groot.CreateSpec{
				ID:            "my-empty",
				BaseImageURL:  integration.String2URL("docker:///cfgarden/empty:v0.1.1"),
				Mount:         false,
				CleanOnCreate: true,
			})
			Expect(err).NotTo(HaveOccurred())

			afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
			Expect(err).NotTo(HaveOccurred())
			Expect(afterContents).To(HaveLen(2))
			for _, layer := range testhelpers.EmptyBaseImageV011.Layers {
				Expect(filepath.Join(StorePath, store.VolumesDirName, layer.ChainID)).To(BeADirectory())
			}
		})

		Context("with local tar image", func() {
			var yetAnotherBaseImagePath string
			BeforeEach(func() {
				yetAnotherSourceImagePath, err := ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())
				yetAnotherBaseImageFile := integration.CreateBaseImageTar(yetAnotherSourceImagePath)
				yetAnotherBaseImagePath = yetAnotherBaseImageFile.Name()

				_, err = Runner.Create(groot.CreateSpec{
					ID:           "my-old-local",
					BaseImageURL: integration.String2URL(yetAnotherBaseImagePath),
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(Runner.Delete("my-old-local")).To(Succeed())

				cmd := exec.Command("touch", yetAnotherBaseImagePath)
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
			})

			It("eventually removes unused local volumes", func() {
				preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(preContents).To(HaveLen(2)) // Docker image and local tar, from 2 BeforeEachs above

				_, err = Runner.Create(groot.CreateSpec{
					ID:            "my-local-1",
					BaseImageURL:  integration.String2URL(yetAnotherBaseImagePath),
					Mount:         false,
					CleanOnCreate: true,
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = Runner.Create(groot.CreateSpec{
					ID:            "my-local-2",
					BaseImageURL:  integration.String2URL(yetAnotherBaseImagePath),
					Mount:         false,
					CleanOnCreate: true,
				})
				Expect(err).NotTo(HaveOccurred())

				afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(afterContents).To(HaveLen(1))
				// We now have 2 groot images, one of which is based on the same base
				// image as the tar rootfs from the preContents, but with a new mtime.
				Expect(preContents).NotTo(ContainElement(afterContents[0]))
			})
		})

		Context("when a threshold is given", func() {
			Context("when the store size is over the threshold", func() {
				It("cleans the store first", func() {
					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
					Expect(err).NotTo(HaveOccurred())
					Expect(preContents).To(HaveLen(1))

					_, err = Runner.Create(groot.CreateSpec{
						ID:                          "my-empty",
						BaseImageURL:                integration.String2URL("docker:///cfgarden/empty:v0.1.1"),
						Mount:                       false,
						CleanOnCreate:               true,
						CleanOnCreateThresholdBytes: 1024,
					})
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(2))
				})
			})

			Context("when thestore size is under the threshold", func() {
				It("cleans the store first", func() {
					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
					Expect(err).NotTo(HaveOccurred())
					Expect(preContents).To(HaveLen(1))

					_, err = Runner.Create(groot.CreateSpec{
						ID:                          "my-empty",
						BaseImageURL:                integration.String2URL("docker:///cfgarden/empty:v0.1.1"),
						Mount:                       false,
						CleanOnCreate:               true,
						CleanOnCreateThresholdBytes: 1024 * 1024 * 1024 * 1024,
					})
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(3))
				})
			})
		})
	})

	Context("when --without-clean is given", func() {
		BeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:           "my-busybox",
				BaseImageURL: integration.String2URL("docker:///cfgarden/garden-busybox"),
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(Runner.Delete("my-busybox")).To(Succeed())
		})

		AfterEach(func() {
			_ = Runner.Delete("my-empty")
		})

		It("does not clean the store", func() {
			preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
			Expect(err).NotTo(HaveOccurred())
			Expect(preContents).To(HaveLen(1))

			_, err = Runner.Create(groot.CreateSpec{
				ID:            "my-empty",
				BaseImageURL:  integration.String2URL("docker:///cfgarden/empty:v0.1.1"),
				Mount:         mountByDefault(),
				CleanOnCreate: false,
			})
			Expect(err).NotTo(HaveOccurred())

			afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
			Expect(err).NotTo(HaveOccurred())
			Expect(afterContents).To(HaveLen(3))

			layers := append(testhelpers.EmptyBaseImageV011.Layers, testhelpers.BusyBoxImage.Layers...)
			for _, layer := range layers {
				Expect(filepath.Join(StorePath, store.VolumesDirName, layer.ChainID)).To(BeADirectory())
			}
		})
	})

	Context("when both without-clean and with-clean flags are given", func() {
		It("returns an error", func() {
			_, err := Runner.WithClean().WithNoClean().Create(groot.CreateSpec{
				ID:           "my-empty",
				BaseImageURL: integration.String2URL("docker:///cfgarden/empty:v0.1.1"),
				Mount:        false,
			})
			Expect(err).To(MatchError(ContainSubstring("with-clean and without-clean cannot be used together")))
		})
	})

	Context("when the id is already being used", func() {
		JustBeforeEach(func() {
			_, err := Runner.Create(groot.CreateSpec{
				ID:           randomImageID,
				BaseImageURL: integration.String2URL(baseImagePath),
				Mount:        false,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("fails and produces a useful error", func() {
			_, err := Runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(baseImagePath),
				ID:           randomImageID,
				Mount:        false,
			})
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("image for id `%s` already exists", randomImageID))))
		})
	})

	Context("when the id is not provided", func() {
		It("fails", func() {
			_, err := Runner.WithStore(StorePath).Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(baseImagePath),
				ID:           "",
				Mount:        false,
			})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the id contains invalid characters", func() {
		It("fails", func() {
			_, err := Runner.WithStore(StorePath).Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(baseImagePath),
				ID:           "this/is/not/okay",
				Mount:        false,
			})
			Expect(err).To(MatchError(ContainSubstring("id `this/is/not/okay` contains invalid characters: `/`")))
		})
	})

	Context("when the requested filesystem driver is not supported", func() {
		It("fails with a useful error message", func() {
			_, err := Runner.WithDriver("dinosaurfs").Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(baseImagePath),
				ID:           "some-id",
				Mount:        true,
			})
			Expect(err).To(MatchError(ContainSubstring("filesystem driver not supported: dinosaurfs")))
		})
	})

	Context("when StorePath doesn't match the given driver", func() {
		var (
			storePath string
			runner    runner.Runner
		)

		BeforeEach(func() {
			var err error
			storePath, err = ioutil.TempDir("/mnt/ext4", "")
			Expect(err).NotTo(HaveOccurred())

			runner = Runner.WithStore(storePath).WithDriver(Driver)
		})

		It("returns an error", func() {
			_, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(baseImagePath),
				ID:           randomImageID,
			})
			errMessage := fmt.Sprintf("Store path filesystem (%s) is incompatible with requested driver", storePath)
			Expect(err).To(MatchError(ContainSubstring(errMessage)))
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
				ID:           randomImageID,
				BaseImageURL: integration.String2URL(baseImagePath),
				Mount:        mountByDefault(),
			}

			Expect(Runner.SetConfig(cfg)).To(Succeed())
		})

		Describe("store path", func() {
			BeforeEach(func() {
				Expect(os.Mkdir(StorePath, 0755)).To(Succeed())
				Expect(os.Chmod(StorePath, 0755)).To(Succeed())
				var err error
				cfg.StorePath, err = ioutil.TempDir(StorePath, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(os.Chmod(cfg.StorePath, 0777)).To(Succeed())
			})

			It("uses the store path from the config file", func() {
				runner := Runner.WithoutStore()
				containerSpec, err := runner.Create(spec)
				Expect(err).NotTo(HaveOccurred())
				Expect(filepath.Dir(containerSpec.Root.Path)).To(Equal(filepath.Join(cfg.StorePath, "images", randomImageID)))
			})
		})

		Describe("filesystem driver", func() {
			BeforeEach(func() {
				cfg.FSDriver = "this-should-fail"
			})

			It("uses the filesystem driver from the config file", func() {
				_, err := Runner.WithoutDriver().Create(spec)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("filesystem driver not supported: this-should-fail"))
			})
		})

		Describe("drax bin", func() {
			var (
				draxCalledFile *os.File
				draxBin        *os.File
			)

			BeforeEach(func() {
				integration.SkipIfNotBTRFS(Driver)
				_, draxBin, draxCalledFile = integration.CreateFakeDrax()
				cfg.DraxBin = draxBin.Name()
			})

			It("uses the drax bin from the config file", func() {
				_, err := Runner.WithoutDraxBin().Create(groot.CreateSpec{
					BaseImageURL: integration.String2URL(baseImagePath),
					ID:           randomImageID,
					DiskLimit:    104857600,
					Mount:        true,
				})
				Expect(err).NotTo(HaveOccurred())

				contents, err := ioutil.ReadFile(draxCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - drax"))
			})
		})

		Describe("tardis bin", func() {
			var (
				tardisCalledFile *os.File
				tardisBin        *os.File
			)

			BeforeEach(func() {
				integration.SkipIfNotXFS(Driver)
				_, tardisBin, tardisCalledFile = integration.CreateFakeTardis()
				cfg.TardisBin = tardisBin.Name()
			})

			It("uses the tardis bin from the config file", func() {
				_, err := Runner.WithoutTardisBin().Create(groot.CreateSpec{
					BaseImageURL: integration.String2URL(baseImagePath),
					ID:           randomImageID,
					DiskLimit:    104857600,
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())

				contents, err := ioutil.ReadFile(tardisCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - tardis"))
			})
		})

		Describe("disk limit size bytes", func() {
			BeforeEach(func() {
				cfg.Create.DiskLimitSizeBytes = tenMegabytes
			})

			It("creates a image with limit from the config file", func() {
				containerSpec, err := Runner.Create(spec)
				Expect(err).ToNot(HaveOccurred())

				Expect(writeMegabytes(filepath.Join(containerSpec.Root.Path, "hello"), 11)).To(MatchError(ContainSubstring("dd: error writing")))
			})
		})

		It("returns a partial runtime-spec as output", func() {
			containerSpec, err := Runner.Create(groot.CreateSpec{
				ID:           randomImageID,
				BaseImageURL: integration.String2URL(baseImagePath),
				Mount:        false,
			})
			Expect(err).ToNot(HaveOccurred())

			expectedRootfs := filepath.Join(StorePath, store.ImageDirName, randomImageID, "rootfs")
			Expect(containerSpec.Root.Path).To(Equal(expectedRootfs))
			Expect(containerSpec.Mounts).To(HaveLen(1))
			Expect(containerSpec.Mounts[0].Destination).To(Equal("/"))
		})

		Describe("without mount", func() {
			It("does not mount the rootfs", func() {
				containerSpec, err := Runner.Create(groot.CreateSpec{
					ID:           "some-id",
					BaseImageURL: integration.String2URL(baseImagePath),
					Mount:        false,
				})
				Expect(err).NotTo(HaveOccurred())

				contents, err := ioutil.ReadDir(containerSpec.Root.Path)
				Expect(err).NotTo(HaveOccurred())
				Expect(contents).To(BeEmpty())
			})

			Describe("Mount json output", func() {
				var (
					containerSpec specs.Spec
				)

				JustBeforeEach(func() {
					var err error
					containerSpec, err = Runner.Create(groot.CreateSpec{
						ID:           "some-id",
						BaseImageURL: integration.String2URL(baseImagePath),
						Mount:        false,
					})
					Expect(err).NotTo(HaveOccurred())
				})

				Context("BTRFS", func() {
					BeforeEach(func() {
						integration.SkipIfNotBTRFS(Driver)
					})

					It("returns the mount information in the output json", func() {
						Expect(containerSpec.Mounts).ToNot(BeNil())
						Expect(containerSpec.Mounts[0].Destination).To(Equal("/"))
						Expect(containerSpec.Mounts[0].Type).To(Equal(""))
						Expect(containerSpec.Mounts[0].Source).To(Equal(filepath.Join(StorePath, store.ImageDirName, "some-id", "snapshot")))
						Expect(containerSpec.Mounts[0].Options).To(HaveLen(1))
						Expect(containerSpec.Mounts[0].Options[0]).To(Equal("bind"))
					})
				})

				Context("XFS", func() {
					BeforeEach(func() {
						integration.SkipIfNotXFS(Driver)
					})

					It("returns the mount information in the output json", func() {
						Expect(containerSpec.Mounts).ToNot(BeNil())
						Expect(containerSpec.Mounts[0].Destination).To(Equal("/"))
						Expect(containerSpec.Mounts[0].Type).To(Equal("overlay"))
						Expect(containerSpec.Mounts[0].Source).To(Equal("overlay"))
						Expect(containerSpec.Mounts[0].Options).To(HaveLen(1))
						Expect(containerSpec.Mounts[0].Options[0]).To(MatchRegexp(
							fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s",
								filepath.Join(StorePath, overlayxfs.LinksDirName, ".*"),
								filepath.Join(StorePath, store.ImageDirName, "some-id", overlayxfs.UpperDir),
								filepath.Join(StorePath, store.ImageDirName, "some-id", overlayxfs.WorkDir),
							),
						))
					})
				})
			})
		})

		Describe("exclude image from quota", func() {
			BeforeEach(func() {
				cfg.Create.ExcludeImageFromQuota = true
				cfg.Create.DiskLimitSizeBytes = tenMegabytes
			})

			It("excludes base image from quota when config property say so", func() {
				containerSpec, err := Runner.Create(spec)
				Expect(err).ToNot(HaveOccurred())
				_, err = Runner.Stats(spec.ID)
				Expect(err).NotTo(HaveOccurred())

				Expect(writeMegabytes(filepath.Join(containerSpec.Root.Path, "hello"), 6)).To(Succeed())
				Expect(writeMegabytes(filepath.Join(containerSpec.Root.Path, "hello2"), 5)).To(MatchError(ContainSubstring("dd: error writing")))
			})
		})
	})
})
