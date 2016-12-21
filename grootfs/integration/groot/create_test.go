package groot_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
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
	var baseImagePath string

	BeforeEach(func() {
		var err error
		baseImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		Expect(ioutil.WriteFile(path.Join(baseImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
	})

	Context("when inclusive disk limit is provided", func() {
		It("creates a image with supplied limit", func() {
			Expect(writeMegabytes(filepath.Join(baseImagePath, "fatfile"), 5)).To(Succeed())

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

						imagePath := path.Join(StorePath, CurrentUserID, "images", "random-id")
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

				Context("when it doesn't exist", func() {
					It("fails early on", func() {
						cmd := exec.Command(GrootFSBin, "--store", StorePath, "--btrfs-bin", "/not-existent", "create", baseImagePath, "random-id")
						sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())
						Eventually(sess).Should(gexec.Exit(1))
						Eventually(sess).Should(gbytes.Say("could not find btrfs binary"))
					})
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
	})

	Context("when no --store option is given", func() {
		It("uses the default store path", func() {
			Expect("/var/lib/grootfs/images").ToNot(BeAnExistingFile())
			_, err := Runner.WithoutStore().Create(groot.CreateSpec{
				BaseImage: baseImagePath,
				ID:        "random-id",
			})
			Expect(err).To(MatchError(ContainSubstring(("making directory `/var/lib/grootfs/" + CurrentUserID + "`"))))
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
		BeforeEach(func() {
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

			Expect(path.Join(StorePath, CurrentUserID, "images", "some-id")).ToNot(BeAnExistingFile())
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
				cfg.BaseStorePath, err = ioutil.TempDir(StorePath, "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("uses the store path from the config file", func() {
				image, err := Runner.WithoutStore().Create(spec)
				Expect(err).NotTo(HaveOccurred())
				Expect(image.Path).To(Equal(filepath.Join(cfg.BaseStorePath, CurrentUserID, "images/random-id")))
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
				Expect(buffer.Contents()).To(ContainSubstring("gid range [1-65991) -> [1-65991) not allowed"))
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
})

func writeMegabytes(outputPath string, mb int) error {
	cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", outputPath), "bs=1048576", fmt.Sprintf("count=%d", mb))
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return err
	}
	Eventually(sess).Should(gexec.Exit())
	if sess.ExitCode() > 0 {
		return errors.New(string(sess.Err.Contents()))
	}
	return nil
}
