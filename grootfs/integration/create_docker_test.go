package integration_test

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	runnerpkg "code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager/v3"

	units "github.com/docker/go-units"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create with remote DOCKER images", func() {
	var (
		randomImageID string
		baseImageURL  *url.URL
		runner        runnerpkg.Runner
	)

	BeforeEach(func() {
		initSpec := runnerpkg.InitSpec{
			UIDMappings: []groot.IDMappingSpec{
				{HostID: GrootUID, NamespaceID: 0, Size: 1},
				{HostID: 100000, NamespaceID: 1, Size: 65000},
			},
			GIDMappings: []groot.IDMappingSpec{
				{HostID: GrootGID, NamespaceID: 0, Size: 1},
				{HostID: 100000, NamespaceID: 1, Size: 65000},
			},
			StoreSizeBytes: 1000 * 1024 * 1024,
		}

		randomImageID = testhelpers.NewRandomID()
		Expect(Runner.RunningAsUser(0, 0).InitStore(initSpec)).To(Succeed())
		runner = Runner.SkipInitStore()
	})

	Context("when using the default registry", func() {
		BeforeEach(func() {
			baseImageURL = integration.String2URL("docker:///cloudfoundry/garden-rootfs")
		})

		It("creates a root filesystem based on the image provided", func() {
			containerSpec, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL("docker:///cloudfoundry/garden-rootfs"),
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(runner.EnsureMounted(containerSpec)).To(Succeed())

			Expect(path.Join(containerSpec.Root.Path, "hello")).To(BeARegularFile())
			Expect(path.Join(containerSpec.Root.Path, "allo")).To(BeARegularFile())
		})

		It("gives any user permission to be inside the container", func() {
			containerSpec, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL("docker:///cloudfoundry/garden-rootfs"),
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(runner.EnsureMounted(containerSpec)).To(Succeed())

			cmd := exec.Command(NamespacerBin, containerSpec.Root.Path, strconv.Itoa(GrootUID+100), "/bin/ls", "/")
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
			}
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 5*time.Second).Should(gexec.Exit(0))
		})

		It("outputs a json with the correct `Root.Path` key", func() {
			containerSpec, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL("docker:///cloudfoundry/garden-rootfs"),
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(containerSpec.Root.Path).To(Equal(filepath.Join(StorePath, store.ImageDirName, randomImageID, "rootfs")))
		})

		It("outputs a json with the correct `Process.Env` key", func() {
			containerSpec, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL("docker:///cloudfoundry/garden-rootfs"),
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(containerSpec.Process.Env).To(ContainElement("PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/bin:/from-dockerfile"))
			Expect(containerSpec.Process.Env).To(ContainElement("TEST=second-test-from-dockerfile:test-from-dockerfile"))
		})

		Context("when the image is bigger than available memory", func() {
			It("doesn't fail", func() {
				sess, err := runner.StartCreate(groot.CreateSpec{
					BaseImageURL: integration.String2URL("docker:///ubuntu:trusty"),
					ID:           "some-id",
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())

				go func() {
					defer GinkgoRecover()

					statsPath := path.Join("/proc", strconv.Itoa(sess.Command.Process.Pid), "status")
					runs := 0
					for {
						stats, err := os.ReadFile(statsPath)
						if err != nil {
							Expect(runs).To(BeNumerically(">", 1))
							break
						}

						var statsMap map[string]string
						Expect(yaml.Unmarshal(stats, &statsMap)).To(Succeed())

						virtualMemoryHighWaterMark := strings.Replace(strings.ToUpper(statsMap["VmHWM"]), " ", "", -1)
						if virtualMemoryHighWaterMark != "" {
							n, err := units.FromHumanSize(virtualMemoryHighWaterMark)
							Expect(err).NotTo(HaveOccurred())
							// Biggest ubuntu:trusty layer is 65694192 bytes
							Expect(n).To(BeNumerically("<", 50*1024*1024))
						}

						time.Sleep(200 * time.Millisecond)
						runs++
					}
				}()

				Expect(sess.Wait(time.Hour * 24).ExitCode()).To(BeZero())
			})
		})

		Context("when the image has volumes which refer to existing directories", func() {
			BeforeEach(func() {
				baseImageURL = integration.String2URL("docker:///cloudfoundry/garden-rootfs")
			})

			It("nothing should be done, but the directories + contents should be visible", func() {
				containerSpec, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(runner.EnsureMounted(containerSpec)).To(Succeed())

				Expect(filepath.Join(StorePath, store.ImageDirName, randomImageID, "rootfs", "etc", "foo")).To(BeADirectory())
			})
		})

		Context("when the image has links that overwrites existing files #overwrite-link", func() {
			BeforeEach(func() {
				baseImageURL = integration.String2URL("docker:///cloudfoundry/garden-rootfs")
			})

			It("creates the link with success", func() {
				containerSpec, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(runner.EnsureMounted(containerSpec)).To(Succeed())

				symlinkFilePath := filepath.Join(containerSpec.Root.Path, "tmp/symlink")
				stat, err := os.Lstat(symlinkFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Mode() & os.ModeSymlink).ToNot(BeZero())
				linkTargetPath, err := os.Readlink(symlinkFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(linkTargetPath).To(Equal("/etc/link-source"))
			})
		})

		Context("when a layer in an image has opaque whiteouts #opaque-whiteouts-regression-image", func() {
			BeforeEach(func() {
				baseImageURL = integration.String2URL("docker:///cloudfoundry/garden-rootfs")
			})

			It("the upper layer dir that contains the opaque whiteout totally shadows the same dir in the lower layer", func() {
				containerSpec, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(runner.EnsureMounted(containerSpec)).To(Succeed())

				whiteoutedDir := filepath.Join(containerSpec.Root.Path, "var")
				contents, err := ioutil.ReadDir(whiteoutedDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(contents).To(HaveLen(1))
				Expect(filepath.Join(containerSpec.Root.Path, "var", "istillexist")).To(BeAnExistingFile())
			})
		})

		Context("when the image has whiteouts", func() {
			BeforeEach(func() {
				baseImageURL = integration.String2URL("docker:///cloudfoundry/garden-rootfs")
			})

			It("removes the whiteout file", func() {
				containerSpec, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(runner.EnsureMounted(containerSpec)).To(Succeed())

				Expect(path.Join(containerSpec.Root.Path, "file-to-be-deleted")).ToNot(BeAnExistingFile())
				Expect(path.Join(containerSpec.Root.Path, "folder")).ToNot(BeAnExistingFile())
				Expect(path.Join(containerSpec.Root.Path, "existing-file")).To(BeAnExistingFile())
			})
		})

		Context("when the image has files with the setuid on", func() {
			BeforeEach(func() {
				baseImageURL = integration.String2URL("docker:///cloudfoundry/garden-rootfs")
			})

			It("correctly applies the user bit", func() {
				containerSpec, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(runner.EnsureMounted(containerSpec)).To(Succeed())

				setuidFilePath := path.Join(containerSpec.Root.Path, "bin", "usemem-with-setuid")
				stat, err := os.Stat(setuidFilePath)
				Expect(err).NotTo(HaveOccurred())

				Expect(stat.Mode() & os.ModeSetuid).To(Equal(os.ModeSetuid))
			})
		})

		Describe("clean up on create", func() {
			JustBeforeEach(func() {
				_, err := runner.Create(groot.CreateSpec{
					ID:           "just-before-each-container",
					BaseImageURL: integration.String2URL("docker:///cloudfoundry/garden-rootfs"),
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(runner.Delete("just-before-each-container")).To(Succeed())
			})

			AfterEach(func() {
				Expect(runner.Delete(randomImageID)).To(Succeed())
			})

			It("cleans up unused layers before create but not the one about to be created", func() {
				volumes, _ := getVolumesDirEntries()
				entries := len(volumes)
				Expect(entries).To(BeNumerically(">", 1))
				_, err := runner.WithClean().Create(groot.CreateSpec{
					ID:           randomImageID,
					BaseImageURL: integration.String2URL("docker:///alpine"),
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Consistently(getVolumesDirEntries).Should(HaveLen(1))

			})

			Context("when no-clean flag is set", func() {
				It("does not clean up unused layers", func() {
					volumes, _ := getVolumesDirEntries()
					entries := len(volumes)
					Expect(entries).To(BeNumerically(">", 1))

					_, err := runner.WithNoClean().Create(groot.CreateSpec{
						ID:           randomImageID,
						BaseImageURL: integration.String2URL("docker:///busybox:1.25"),
						Mount:        mountByDefault(),
					})
					Expect(err).NotTo(HaveOccurred())

					Consistently(getVolumesDirEntries).Should(HaveLen(entries + 1))
				})
			})
		})

		Context("when the image is private", func() {
			BeforeEach(func() {
				baseImageURL = integration.String2URL("docker:///cloudfoundry/garden-private-image-test")
			})

			Context("when the credentials are correct", func() {
				It("succeeds", func() {
					runner := runner.WithCredentials(RegistryUsername, RegistryPassword)
					containerSpec, err := runner.Create(groot.CreateSpec{
						BaseImageURL: baseImageURL,
						ID:           randomImageID,
						Mount:        mountByDefault(),
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(runner.EnsureMounted(containerSpec)).To(Succeed())
				})
			})

			Context("when the credentials are wrong", func() {
				// We need a fake registry here because Dockerhub was rate limiting on multiple bad credential auth attempts
				var fakeRegistry *testhelpers.FakeRegistry

				BeforeEach(func() {
					dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
					Expect(err).NotTo(HaveOccurred())
					fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
					fakeRegistry.Start()
					fakeRegistry.ForceTokenAuthError()
					baseImageURL = integration.String2URL(fmt.Sprintf("docker://%s/doesnt-matter-because-fake-registry", fakeRegistry.Addr()))
				})

				AfterEach(func() {
					fakeRegistry.Stop()
				})

				It("fails", func() {
					runner := runner.WithCredentials("someuser", "invalid-password").WithInsecureRegistry(fakeRegistry.Addr())
					_, err := runner.Create(groot.CreateSpec{
						BaseImageURL: baseImageURL,
						ID:           randomImageID,
						Mount:        mountByDefault(),
					})
					Expect(err).To(MatchError(ContainSubstring("unable to retrieve auth token: invalid username/password")))
				})
			})

			It("does not log the credentials OR their references", func() {
				buffer := gbytes.NewBuffer()
				runner := runner.WithCredentials(RegistryUsername, RegistryPassword).WithStderr(buffer).WithLogLevel(lager.DEBUG)
				_, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).ToNot(HaveOccurred())
				Eventually(buffer).ShouldNot(gbytes.Say("\"RegistryUsername\":\"\",\"RegistryPassword\":\"\""))
				Eventually(buffer).ShouldNot(gbytes.Say(RegistryPassword))
			})
		})

		Context("when the total size of compressed layers is greater than the quota", func() {
			BeforeEach(func() {
				baseImageURL = integration.String2URL("docker:///cloudfoundry/garden-rootfs")
			})

			Context("when the image is not accounted for in the quota", func() {
				It("succeeds", func() {
					containerSpec, err := runner.Create(groot.CreateSpec{
						BaseImageURL:              baseImageURL,
						ID:                        randomImageID,
						Mount:                     mountByDefault(),
						ExcludeBaseImageFromQuota: true,
						DiskLimit:                 10,
					})
					Expect(runner.EnsureMounted(containerSpec)).To(Succeed())
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when the image is accounted for in the quota", func() {
				It("returns an error", func() {
					_, err := runner.Create(groot.CreateSpec{
						BaseImageURL: baseImageURL,
						ID:           randomImageID,
						Mount:        mountByDefault(),
						DiskLimit:    10,
					})
					Expect(err).To(MatchError(ContainSubstring("layers exceed disk quota")))
				})
			})
		})

		Context("when the total size of compressed layer is less than the quota, but the uncompressed size is bigger", func() {
			var diskLimit int64 = 7 * 1024 * 1024

			BeforeEach(func() {
				baseImageURL = integration.String2URL("docker:///cloudfoundry/garden-rootfs")
			})

			It("returns an error", func() {
				_, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
					DiskLimit:    diskLimit,
				})
				Expect(err).To(MatchError(ContainSubstring("uncompressed layer size exceeds quota")))
			})

			Context("when the image is not accounted for in the quota", func() {
				It("succeeds", func() {
					containerSpec, err := runner.Create(groot.CreateSpec{
						BaseImageURL:              baseImageURL,
						ID:                        randomImageID,
						Mount:                     mountByDefault(),
						ExcludeBaseImageFromQuota: true,
						DiskLimit:                 diskLimit,
					})
					Expect(runner.EnsureMounted(containerSpec)).To(Succeed())
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Describe("Unpacked layer caching", func() {
			BeforeEach(func() {
				baseImageURL = integration.String2URL("docker:///cloudfoundry/garden-rootfs")
			})
			It("caches the unpacked image as a volume", func() {
				_, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).ToNot(HaveOccurred())
				volumesDir := filepath.Join(StorePath, "volumes")

				dirs, err := os.ReadDir(volumesDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(dirs)).NotTo(BeZero())

				layerSnapshotPath := filepath.Join(volumesDir, dirs[0].Name())
				Expect(os.WriteFile(layerSnapshotPath+"/injected-file", []byte{}, 0666)).To(Succeed())

				containerSpec, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           testhelpers.NewRandomID(),
					Mount:        mountByDefault(),
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(runner.EnsureMounted(containerSpec)).To(Succeed())
				Expect(path.Join(containerSpec.Root.Path, "hello")).To(BeARegularFile())
				Expect(path.Join(containerSpec.Root.Path, "injected-file")).To(BeARegularFile())
			})

			Describe("when one of the layers is corrupted", func() {
				var (
					fakeRegistry  *testhelpers.FakeRegistry
					corruptedBlob string
					volumesDir    string
				)

				BeforeEach(func() {
					volumesDir = filepath.Join(StorePath, store.VolumesDirName)
					dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
					Expect(err).NotTo(HaveOccurred())

					fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
					fakeRegistry.Start()
				})

				AfterEach(func() {
					fakeRegistry.Stop()
				})

				Context("when the image has a version 2 manifest schema", func() {
					BeforeEach(func() {
						baseImageURL = integration.String2URL(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
						corruptedBlob = testhelpers.EmptyBaseImageV011.Layers[1].BlobID
						fakeRegistry.WhenGettingBlob(corruptedBlob, 0, func(w http.ResponseWriter, r *http.Request) {
							gzipWriter := gzip.NewWriter(w)
							_, err := io.WriteString(gzipWriter, "bad-blob")
							gzipWriter.Close()
							Expect(err).NotTo(HaveOccurred())
						})
					})

					It("fails and cleans up the corrupted volumes", func() {
						runner := runner.WithInsecureRegistry(fakeRegistry.Addr())

						_, err := runner.Create(groot.CreateSpec{
							BaseImageURL: baseImageURL,
							ID:           randomImageID,
							Mount:        mountByDefault(),
						})

						Expect(err).To(MatchError(ContainSubstring("layer size is different from the value in the manifest")))

						volumes, _ := ioutil.ReadDir(volumesDir)
						Expect(len(volumes)).To(Equal(len(testhelpers.EmptyBaseImageV011.Layers) - 1))

						Expect(filepath.Join(volumesDir, testhelpers.EmptyBaseImageV011.Layers[0].ChainID)).To(BeADirectory())
						Expect(filepath.Join(volumesDir, testhelpers.EmptyBaseImageV011.Layers[1].ChainID)).ToNot(BeADirectory())
					})
				})

				Context("when the image has a version 1 manifest schema", func() {
					BeforeEach(func() {
						baseImageURL = integration.String2URL(fmt.Sprintf("docker://%s/cfgarden/empty:schemaV1", fakeRegistry.Addr()))
						corruptedBlob = testhelpers.SchemaV1EmptyBaseImage.Layers[2].BlobID
						fakeRegistry.WhenGettingBlob(corruptedBlob, 0, func(w http.ResponseWriter, r *http.Request) {
							_, err := io.WriteString(w, "bad-blob")
							Expect(err).NotTo(HaveOccurred())
						})
					})

					It("fails and never creates any volumes", func() {
						runner := runner.WithInsecureRegistry(fakeRegistry.Addr())

						_, err := runner.Create(groot.CreateSpec{
							BaseImageURL: baseImageURL,
							ID:           randomImageID,
							Mount:        mountByDefault(),
						})

						Expect(err).To(MatchError(ContainSubstring("converting V1 schema failed")))

						volumes, err := ioutil.ReadDir(volumesDir)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(volumes)).To(Equal(0))
					})
				})
			})
		})

		Context("when the image has a version 1 manifest schema", func() {
			BeforeEach(func() {
				baseImageURL = integration.String2URL("docker:///cfgarden/empty:schemaV1")
			})

			It("creates a root filesystem based on the image provided", func() {
				containerSpec, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(runner.EnsureMounted(containerSpec)).To(Succeed())
				Expect(path.Join(containerSpec.Root.Path, "allo")).To(BeAnExistingFile())
				Expect(path.Join(containerSpec.Root.Path, "hello")).To(BeAnExistingFile())

				volumesDir := filepath.Join(StorePath, store.VolumesDirName)
				Expect(filepath.Join(volumesDir, testhelpers.SchemaV1EmptyBaseImage.Layers[0].ChainID)).To(BeADirectory())
				Expect(filepath.Join(volumesDir, testhelpers.SchemaV1EmptyBaseImage.Layers[1].ChainID)).To(BeADirectory())
				Expect(filepath.Join(volumesDir, testhelpers.SchemaV1EmptyBaseImage.Layers[2].ChainID)).To(BeADirectory())
			})
		})
	})

	Context("when a private registry is used", func() {
		var fakeRegistry *testhelpers.FakeRegistry

		BeforeEach(func() {
			dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
			Expect(err).NotTo(HaveOccurred())
			fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
			fakeRegistry.Start()

			baseImageURL = integration.String2URL(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
		})

		AfterEach(func() {
			fakeRegistry.Stop()
		})

		It("fails to fetch the image", func() {
			_, err := runner.Create(groot.CreateSpec{
				BaseImageURL: baseImageURL,
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).To(MatchError("This registry is insecure. To pull images from this registry, please use the --insecure-registry option."))
		})

		Context("when it's provided as a valid insecure registry", func() {
			It("should create a root filesystem based on the image provided by the private registry", func() {
				runner := runner.WithInsecureRegistry(fakeRegistry.Addr())
				containerSpec, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(runner.EnsureMounted(containerSpec)).To(Succeed())

				Expect(path.Join(containerSpec.Root.Path, "hello")).To(BeARegularFile())
				Expect(fakeRegistry.RequestedBlobs()).To(HaveLen(3))
			})
		})

		Context("when --config global flag is given", func() {
			Describe("with an insecure registries list", func() {
				var (
					configDir      string
					configFilePath string
				)

				BeforeEach(func() {
					var err error
					configDir, err = ioutil.TempDir("", "")
					Expect(err).NotTo(HaveOccurred())
					Expect(os.Chmod(configDir, 0755)).To(Succeed())

					cfg := config.Config{
						Create: config.Create{
							InsecureRegistries: []string{fakeRegistry.Addr()},
						},
					}

					configYaml, err := yaml.Marshal(cfg)
					Expect(err).NotTo(HaveOccurred())
					configFilePath = path.Join(configDir, "config.yaml")

					Expect(ioutil.WriteFile(configFilePath, configYaml, 0755)).To(Succeed())
					Expect(os.Chmod(configFilePath, 0755)).To(Succeed())
				})

				AfterEach(func() {
					Expect(os.RemoveAll(configDir)).To(Succeed())
				})

				It("creates a root filesystem based on the image provided by the private registry", func() {
					runner := runner.WithConfig(configFilePath)
					containerSpec, err := runner.Create(groot.CreateSpec{
						BaseImageURL: baseImageURL,
						ID:           randomImageID,
						Mount:        mountByDefault(),
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(runner.EnsureMounted(containerSpec)).To(Succeed())

					Expect(path.Join(containerSpec.Root.Path, "hello")).To(BeARegularFile())
					Expect(fakeRegistry.RequestedBlobs()).To(HaveLen(3))
				})
			})

			Context("when config path is invalid", func() {
				It("returns a useful error", func() {
					runner := runner.WithConfig("invalid-config-path")
					_, err := runner.Create(groot.CreateSpec{
						BaseImageURL: baseImageURL,
						ID:           randomImageID,
						Mount:        mountByDefault(),
					})
					Expect(err).To(MatchError(ContainSubstring("invalid config path")))
				})
			})
		})
	})

	Context("when the image does not exist", func() {
		It("returns a useful error", func() {
			_, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL("docker:///cfgaren/sorry-not-here"),
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).To(MatchError(ContainSubstring("cfgaren/sorry-not-here")))
			Expect(err).To(MatchError(ContainSubstring("requested access to the resource is denied")))
		})
	})

	Context("when the image has files that are not writable to their owner", func() {
		BeforeEach(func() {
			baseImageURL = integration.String2URL("docker:///cloudfoundry/garden-rootfs")
		})
		Context("id mappings", func() {
			It("works", func() {
				containerSpec, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(runner.EnsureMounted(containerSpec)).To(Succeed())

				Expect(path.Join(containerSpec.Root.Path, "test", "hello")).To(BeARegularFile())
			})
		})
	})

	Context("when the image has folders that are not writable to their owner", func() {
		BeforeEach(func() {
			baseImageURL = integration.String2URL("docker:///cloudfoundry/garden-rootfs")
		})
		It("works", func() {
			containerSpec, err := runner.Create(groot.CreateSpec{
				BaseImageURL: baseImageURL,
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(runner.EnsureMounted(containerSpec)).To(Succeed())
			Expect(path.Join(containerSpec.Root.Path, "test", "hello")).To(BeARegularFile())
		})
	})

	Context("when --skip-layer-validation flag is passed", func() {
		var (
			fakeRegistry  *testhelpers.FakeRegistry
			corruptedBlob string
		)

		AfterEach(func() {
			fakeRegistry.Stop()
		})

		BeforeEach(func() {
			dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
			Expect(err).NotTo(HaveOccurred())
			fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
			corruptedBlob = testhelpers.EmptyBaseImageV011.Layers[1].BlobID
			fakeRegistry.WhenGettingBlob(corruptedBlob, 0, func(w http.ResponseWriter, r *http.Request) {
				gzipWriter := gzip.NewWriter(w)
				_, err := io.WriteString(gzipWriter, "bad-blob")
				gzipWriter.Close()
				Expect(err).NotTo(HaveOccurred())
			})
			fakeRegistry.Start()
			baseImageURL = integration.String2URL(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
		})

		It("has no effect", func() {
			r := runner.WithInsecureRegistry(fakeRegistry.Addr())
			_, err := r.SkipLayerCheckSumValidation().Create(groot.CreateSpec{
				BaseImageURL: baseImageURL,
				ID:           randomImageID,
				Mount:        true,
			})

			Expect(err).To(MatchError(ContainSubstring("layer size is different from the value in the manifest")))
		})
	})

	Context("when an image has an invalid DiffID", func() {
		It("fails to create the rootfs", func() {
			_, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL("docker:///cfgarden/alpine-invalid-diffid"),
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).To(MatchError(ContainSubstring("diffID digest mismatch")))
		})

		It("doesn't pollute the cache with invalid layers", func() {
			_, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL("docker:///cfgarden/alpine-invalid-diffid"),
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})

			Expect(err).To(HaveOccurred())
			Expect(filepath.Join(StorePath, store.VolumesDirName, "0000000000000000000000000000000000000000000000000000000000000000")).ToNot(BeAnExistingFile())
		})
	})
})
