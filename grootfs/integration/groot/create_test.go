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
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
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
			cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(baseImagePath, "fatfile")), "bs=1048576", "count=5")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id", int64(10*1024*1024))

			cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(image.RootFSPath, "hello")), "bs=1048576", "count=4")
			sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(image.RootFSPath, "hello2")), "bs=1048576", "count=2")
			sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Expect(sess.Err).To(gbytes.Say("Disk quota exceeded"))
		})

		Context("when the disk limit value is invalid", func() {
			It("fails with a helpful error", func() {
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "--drax-bin", DraxBin, "create", "--disk-limit-size-bytes", "-200", baseImagePath, "random-id")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))
				Eventually(sess).Should(gbytes.Say("disk limit cannot be negative"))
			})
		})

		Context("when the exclude-image-from-quota is also provided", func() {
			It("creates a image with supplied limit, but doesn't take into account the base image size", func() {
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "--drax-bin", DraxBin, "create", "--disk-limit-size-bytes", "10485760", "--exclude-image-from-quota", baseImagePath, "random-id")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				rootfsPath := filepath.Join(StorePath, CurrentUserID, "images/random-id/rootfs")
				cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(rootfsPath, "hello")), "bs=1048576", "count=6")
				sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(rootfsPath, "hello2")), "bs=1048576", "count=5")
				sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))
				Expect(sess.Err).To(gbytes.Say("Disk quota exceeded"))
			})
		})

		Describe("--config global flag", func() {
			var (
				configDir         string
				configFilePath    string
				configStorePath   string
				configDraxBinPath string
				configUIDMappings []string
			)

			BeforeEach(func() {
				configStorePath = StorePath
				configDraxBinPath = ""
				configUIDMappings = nil
			})

			JustBeforeEach(func() {
				var err error
				configDir, err = ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())

				cfg := config.Config{
					BaseStorePath: configStorePath,
					DraxBin:       configDraxBinPath,
					UIDMappings:   configUIDMappings,
				}

				configYaml, err := yaml.Marshal(cfg)
				Expect(err).NotTo(HaveOccurred())
				configFilePath = path.Join(configDir, "config.yaml")

				Expect(ioutil.WriteFile(configFilePath, configYaml, 0755)).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.RemoveAll(configDir)).To(Succeed())
			})

			Describe("store path", func() {
				var (
					runner runnerpkg.Runner
					spec   groot.CreateSpec
				)

				BeforeEach(func() {
					var err error
					configStorePath, err = ioutil.TempDir(StorePath, "")
					Expect(err).NotTo(HaveOccurred())
				})

				JustBeforeEach(func() {
					runner = runnerpkg.Runner{
						GrootFSBin: GrootFSBin,
						DraxBin:    DraxBin,
					}.WithLogLevel(lager.DEBUG).WithStderr(GinkgoWriter).WithConfig(configFilePath)
					spec = groot.CreateSpec{
						ID:        "random-id",
						BaseImage: baseImagePath,
					}
				})

				It("uses the store path from the config file", func() {
					image, err := runner.Create(spec)
					Expect(err).NotTo(HaveOccurred())
					Expect(image.Path).To(Equal(filepath.Join(configStorePath, CurrentUserID, "images/random-id")))
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
					_, err := runner.Create(groot.CreateSpec{
						BaseImage: baseImagePath,
						ID:        "random-id",
						DiskLimit: 104857600,
					})
					Expect(err).NotTo(HaveOccurred())

					contents, err := ioutil.ReadFile(draxCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot"))
				})
			})

			Describe("uid mappings", func() {
				var (
					runner runnerpkg.Runner
					spec   groot.CreateSpec
				)

				BeforeEach(func() {
					configUIDMappings = []string{"1:1:65990"}
				})

				JustBeforeEach(func() {
					runner = runnerpkg.Runner{
						GrootFSBin: GrootFSBin,
						DraxBin:    DraxBin,
						StorePath:  StorePath,
					}.WithLogLevel(lager.DEBUG).WithStderr(GinkgoWriter).WithConfig(configFilePath)
					spec = groot.CreateSpec{
						ID:        "some-id",
						BaseImage: baseImagePath,
					}
				})

				It("uses the uid mappings from the config file", func() {
					buffer := gbytes.NewBuffer()
					_, err := runner.WithStdout(buffer).Create(spec)
					Expect(err).To(HaveOccurred())
					Expect(buffer.Contents()).To(ContainSubstring("uid range [1-65991) -> [1-65991) not allowed"))
				})
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
					cmd := exec.Command(GrootFSBin, "--store", StorePath, "--drax-bin", draxBin.Name(), "create", "--disk-limit-size-bytes", "104857600", baseImagePath, "random-id")
					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					contents, err := ioutil.ReadFile(draxCalledFile.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("I'm groot"))
				})

				Context("when the drax bin doesn't have uid bit set", func() {
					It("doesn't leak the image dir", func() {
						testhelpers.UnsuidDrax(draxBin.Name())
						cmd := exec.Command(GrootFSBin, "--log-level", "debug", "--store", StorePath, "--drax-bin", draxBin.Name(), "create", "--disk-limit-size-bytes", "104857600", baseImagePath, "random-id")
						sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())
						Eventually(sess).Should(gexec.Exit(1))

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
					Expect(string(contents)).To(Equal("I'm groot"))
				})
			})
		})
	})

	Context("when no --store option is given", func() {
		It("uses the default store path", func() {
			Expect("/var/lib/grootfs/images").ToNot(BeAnExistingFile())

			cmd := exec.Command(GrootFSBin, "create", baseImagePath, "random-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			// It will fail at this point, because /var/lib/grootfs doesn't exist
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess).Should(gbytes.Say("making directory `/var/lib/grootfs/" + CurrentUserID + "`"))
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
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", baseImagePath, "random-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Eventually(sess.Out).Should(gbytes.Say("image for id `random-id` already exists"))
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
		})
	})

	Context("when the id is not provided", func() {
		It("fails", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", baseImagePath)
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
		})
	})

	Context("when the id contains invalid characters", func() {
		It("fails", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", baseImagePath, "this/is/not/okay")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Out).Should(gbytes.Say("id `this/is/not/okay` contains invalid characters: `/`"))
		})
	})

	Context("when groot does not have permissions to apply the requested mapping", func() {
		It("returns the newuidmap output in the stdout", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"create",
				"--uid-mapping", "1:1:65000",
				baseImagePath,
				"some-id",
			)

			buffer := gbytes.NewBuffer()
			sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).NotTo(gexec.Exit(0))

			Eventually(buffer).Should(gbytes.Say(`range [\[\)0-9\-]* -> [\[\)0-9\-]* not allowed`))
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
})
