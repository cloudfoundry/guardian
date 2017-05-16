package integration_test

import (
	"encoding/json"
	"fmt"
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

	digestpkg "github.com/opencontainers/go-digest"
	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager"

	"github.com/alecthomas/units"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Create with remote images", func() {
	BeforeEach(func() {
		integration.SkipIfNonRootAndNotBTRFS(GrootfsTestUid, Driver)
	})

	var baseImageURL string

	Context("when using the default registry", func() {
		BeforeEach(func() {
			baseImageURL = "docker:///cfgarden/empty:v0.1.0"
		})

		It("creates a root filesystem based on the image provided", func() {
			image, err := Runner.Create(groot.CreateSpec{
				BaseImage: "docker:///cfgarden/three-layers",
				ID:        "random-id",
				Mount:     true,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(path.Join(image.Rootfs, "layer-3-file")).To(BeARegularFile())
			Expect(path.Join(image.Rootfs, "layer-2-file")).To(BeARegularFile())
			Expect(path.Join(image.Rootfs, "layer-1-file")).To(BeARegularFile())
		})

		It("saves the image.json to the image folder", func() {
			image, err := Runner.Create(groot.CreateSpec{
				BaseImage: baseImageURL,
				ID:        "random-id",
				Mount:     true,
			})
			Expect(err).NotTo(HaveOccurred())

			imageJsonPath := path.Join(image.Path, "image.json")
			Expect(imageJsonPath).To(BeARegularFile())

			imageJsonReader, err := os.Open(imageJsonPath)
			Expect(err).ToNot(HaveOccurred())
			var imageJson specsv1.Image
			Expect(json.NewDecoder(imageJsonReader).Decode(&imageJson)).To(Succeed())

			Expect(imageJson.Created.String()).To(Equal("2016-08-03 16:50:55.797615406 +0000 UTC"))
			Expect(imageJson.RootFS.DiffIDs).To(Equal([]digestpkg.Digest{
				digestpkg.NewDigestFromHex("sha256", "3355e23c079e9b35e4b48075147a7e7e1850b99e089af9a63eed3de235af98ca"),
			}))
		})

		It("gives any user permission to be inside the container", func() {
			image, err := Runner.Create(groot.CreateSpec{
				BaseImage: "docker:///cfgarden/garden-busybox",
				ID:        "random-id",
				Mount:     true,
				UIDMappings: []groot.IDMappingSpec{
					{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
					{HostID: 100000, NamespaceID: 1, Size: 65000},
				},
				GIDMappings: []groot.IDMappingSpec{
					{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
					{HostID: 100000, NamespaceID: 1, Size: 65000},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(NamespacerBin, image.Rootfs, strconv.Itoa(int(GrootUID+100)), "/bin/ls", "/")
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
			}
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})

		It("outputs a json with the correct `rootfs` key", func() {
			image, err := Runner.Create(groot.CreateSpec{
				BaseImage: "docker:///cfgarden/garden-busybox",
				ID:        "random-id",
				Mount:     true,
				UIDMappings: []groot.IDMappingSpec{
					{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
					{HostID: 100000, NamespaceID: 1, Size: 65000},
				},
				GIDMappings: []groot.IDMappingSpec{
					{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
					{HostID: 100000, NamespaceID: 1, Size: 65000},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(image.Rootfs).To(Equal(filepath.Join(StorePath, store.ImageDirName, "random-id", "rootfs")))
		})

		It("outputs a json with the correct `config` key", func() {
			image, err := Runner.Create(groot.CreateSpec{
				BaseImage: "docker:///cfgarden/garden-busybox",
				ID:        "random-id",
				Mount:     true,
				UIDMappings: []groot.IDMappingSpec{
					{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
					{HostID: 100000, NamespaceID: 1, Size: 65000},
				},
				GIDMappings: []groot.IDMappingSpec{
					{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
					{HostID: 100000, NamespaceID: 1, Size: 65000},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(image.Image.RootFS.DiffIDs[0]).To(Equal(digestpkg.NewDigestFromHex("sha256", testhelpers.BusyBoxImage.Layers[0].ChainID)))
		})

		Context("when the image is bigger than available memory", func() {
			It("doesn't fail", func() {
				sess, err := Runner.StartCreate(groot.CreateSpec{
					BaseImage: "docker:///ubuntu:trusty",
					ID:        "some-id",
					Mount:     true,
				})
				Expect(err).NotTo(HaveOccurred())

				go func() {
					defer GinkgoRecover()

					statsPath := path.Join("/proc", strconv.Itoa(sess.Command.Process.Pid), "status")
					runs := 0
					for {
						stats, err := ioutil.ReadFile(statsPath)
						if err != nil {
							Expect(runs).To(BeNumerically(">", 1))
							break
						}

						var statsMap map[string]string
						Expect(yaml.Unmarshal(stats, &statsMap)).To(Succeed())

						virtualMemoryHighWaterMark := strings.Replace(strings.ToUpper(statsMap["VmHWM"]), " ", "", -1)
						if virtualMemoryHighWaterMark != "" {
							n, err := units.ParseBase2Bytes(virtualMemoryHighWaterMark)
							Expect(err).NotTo(HaveOccurred())
							// Biggest ubuntu:trusty layer is 65694192 bytes
							Expect(n).To(BeNumerically("<", 50*1024*1024))
						}

						time.Sleep(200 * time.Millisecond)
						runs++
					}
				}()

				Eventually(sess, 180*time.Second).Should(gexec.Exit(0))
			})
		})

		Context("when the image has volumes", func() {
			BeforeEach(func() {
				baseImageURL = "docker:///cfgarden/with-volume"
			})

			It("creates the volume folders", func() {
				image, err := Runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     true,
				})
				Expect(err).NotTo(HaveOccurred())
				volumeFolder := path.Join(image.Rootfs, "foo")
				Expect(volumeFolder).To(BeADirectory())
			})
		})

		Context("when the image has links that overwrites existing files", func() {
			BeforeEach(func() {
				baseImageURL = "docker:///cfgarden/overwrite-link"
			})

			It("creates the link with success", func() {
				image, err := Runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     true,
				})
				Expect(err).NotTo(HaveOccurred())

				symlinkFilePath := filepath.Join(image.Rootfs, "tmp/symlink")
				stat, err := os.Lstat(symlinkFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Mode() & os.ModeSymlink).ToNot(BeZero())
				linkTargetPath, err := os.Readlink(symlinkFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(linkTargetPath).To(Equal("/etc/link-source"))
			})
		})

		Context("when the image has opaque white outs", func() {
			BeforeEach(func() {
				baseImageURL = "docker:///cfgarden/opq-whiteout-busybox"
			})

			It("empties the folder contents but keeps the dir", func() {
				image, err := Runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     true,
					UIDMappings: []groot.IDMappingSpec{
						{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
						{HostID: 100000, NamespaceID: 1, Size: 65000},
					},
					GIDMappings: []groot.IDMappingSpec{
						{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
						{HostID: 100000, NamespaceID: 1, Size: 65000},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				whiteoutedDir := path.Join(image.Rootfs, "var")
				Expect(whiteoutedDir).To(BeADirectory())
				contents, err := ioutil.ReadDir(whiteoutedDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(contents).To(BeEmpty())
			})
		})

		Context("when the image has whiteouts", func() {
			BeforeEach(func() {
				baseImageURL = "docker:///cfgarden/with-whiteouts"
			})

			It("removes the whiteout file", func() {
				image, err := Runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     true,
					UIDMappings: []groot.IDMappingSpec{
						{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
						{HostID: 100000, NamespaceID: 1, Size: 65000},
					},
					GIDMappings: []groot.IDMappingSpec{
						{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
						{HostID: 100000, NamespaceID: 1, Size: 65000},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(path.Join(image.Rootfs, "file-to-be-deleted")).ToNot(BeAnExistingFile())
				Expect(path.Join(image.Rootfs, "folder")).ToNot(BeAnExistingFile())
				Expect(path.Join(image.Rootfs, "existing-file")).To(BeAnExistingFile())
			})
		})

		Context("when the image has files with the setuid on", func() {
			BeforeEach(func() {
				baseImageURL = "docker:///cfgarden/garden-busybox"
			})

			It("correctly applies the user bit", func() {
				image, err := Runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     true,
				})
				Expect(err).NotTo(HaveOccurred())

				setuidFilePath := path.Join(image.Rootfs, "bin", "busybox")
				stat, err := os.Stat(setuidFilePath)
				Expect(err).NotTo(HaveOccurred())

				Expect(stat.Mode() & os.ModeSetuid).To(Equal(os.ModeSetuid))
			})
		})

		Describe("clean up on create", func() {
			var (
				imageID string
			)

			JustBeforeEach(func() {
				_, err := Runner.Create(groot.CreateSpec{
					ID:        "my-busybox",
					BaseImage: "docker:///cfgarden/garden-busybox",
					Mount:     true,
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
					Mount:     true,
				}
				_, err := Runner.Create(createSpec)
				Expect(err).NotTo(HaveOccurred())
				Expect(runner.Delete("my-empty")).To(Succeed())

				layerPath := filepath.Join(StorePath, store.VolumesDirName, testhelpers.EmptyBaseImageV011.Layers[0].ChainID)
				stat, err := os.Stat(layerPath)
				Expect(err).NotTo(HaveOccurred())
				preLayerTimestamp := stat.ModTime()

				preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(preContents).To(HaveLen(3))

				_, err = runner.Create(groot.CreateSpec{
					ID:        imageID,
					BaseImage: "docker:///cfgarden/empty:v0.1.1",
					Mount:     true,
				})
				Expect(err).NotTo(HaveOccurred())

				afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(afterContents).To(HaveLen(2))

				for _, layer := range testhelpers.EmptyBaseImageV011.Layers {
					Expect(filepath.Join(StorePath, store.VolumesDirName, layer.ChainID)).To(BeADirectory())
				}

				stat, err = os.Stat(layerPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.ModTime()).To(Equal(preLayerTimestamp))
			})

			Context("when no-clean flag is set", func() {
				It("does not clean up unused layers", func() {
					preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
					Expect(err).NotTo(HaveOccurred())
					Expect(preContents).To(HaveLen(1))

					_, err = Runner.WithNoClean().Create(groot.CreateSpec{
						ID:        imageID,
						BaseImage: "docker:///cfgarden/empty:v0.1.1",
						Mount:     true,
					})
					Expect(err).NotTo(HaveOccurred())

					afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
					Expect(err).NotTo(HaveOccurred())
					Expect(afterContents).To(HaveLen(3))
				})
			})
		})

		Context("when the image is private", func() {
			BeforeEach(func() {
				baseImageURL = "docker:///cfgarden/private"
			})

			Context("when the credentials are correct", func() {
				It("succeeds", func() {
					runner := Runner.WithCredentials(RegistryUsername, RegistryPassword)
					_, err := runner.Create(groot.CreateSpec{
						BaseImage: baseImageURL,
						ID:        "random-id",
						Mount:     true,
					})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the credentials are wrong", func() {
				It("fails", func() {
					runner := Runner.WithCredentials("someuser", "invalid-password")
					_, err := runner.Create(groot.CreateSpec{
						BaseImage: baseImageURL,
						ID:        "random-id",
						Mount:     true,
					})
					Expect(err).To(MatchError(ContainSubstring("authorization failed: username and password are invalid")))
				})
			})

			It("does not log the credentials OR their references", func() {
				buffer := gbytes.NewBuffer()
				runner := Runner.WithCredentials(RegistryUsername, RegistryPassword).WithStderr(buffer).WithLogLevel(lager.DEBUG)
				_, err := runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     true,
				})
				Expect(err).ToNot(HaveOccurred())
				Eventually(buffer).ShouldNot(gbytes.Say("\"RegistryUsername\":\"\",\"RegistryPassword\":\"\""))
				Eventually(buffer).ShouldNot(gbytes.Say(RegistryPassword))
			})
		})

		Context("when image size exceeds disk quota", func() {
			BeforeEach(func() {
				baseImageURL = "docker:///cfgarden/empty:v0.1.1"
			})

			Context("when the image is not accounted for in the quota", func() {
				It("succeeds", func() {
					_, err := Runner.Create(groot.CreateSpec{
						BaseImage: baseImageURL,
						ID:        "random-id",
						Mount:     mountByDefault(),
						ExcludeBaseImageFromQuota: true,
						DiskLimit:                 10,
					})
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when the image is accounted for in the quota", func() {
				It("returns an error", func() {
					_, err := Runner.Create(groot.CreateSpec{
						BaseImage: baseImageURL,
						ID:        "random-id",
						Mount:     mountByDefault(),
						DiskLimit: 10,
					})
					Expect(err).To(MatchError(ContainSubstring("layers exceed disk quota")))
				})
			})
		})

		Describe("Unpacked layer caching", func() {
			It("caches the unpacked image as a volume", func() {
				_, err := Runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     true,
				})
				Expect(err).ToNot(HaveOccurred())

				layerSnapshotPath := filepath.Join(StorePath, "volumes", "3355e23c079e9b35e4b48075147a7e7e1850b99e089af9a63eed3de235af98ca")
				Expect(ioutil.WriteFile(layerSnapshotPath+"/injected-file", []byte{}, 0666)).To(Succeed())

				image, err := Runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id-2",
					Mount:     true,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(path.Join(image.Rootfs, "hello")).To(BeARegularFile())
				Expect(path.Join(image.Rootfs, "injected-file")).To(BeARegularFile())
			})

			Describe("when unpacking the image fails", func() {
				var fakeRegistry *testhelpers.FakeRegistry

				BeforeEach(func() {
					dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
					Expect(err).NotTo(HaveOccurred())
					fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
					fakeRegistry.WhenGettingBlob("6c1f4533b125f8f825188c4f4ff633a338cfce0db2813124d3d518028baf7d7a", 0, func(w http.ResponseWriter, r *http.Request) {
						w.Write([]byte("bad-blob"))
					})
					Expect(fakeRegistry.Start()).To(Succeed())

					baseImageURL = fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.0", fakeRegistry.Addr())
				})

				AfterEach(func() {
					fakeRegistry.Stop()
				})

				It("deletes the layer volume cache", func() {
					runner := Runner.WithInsecureRegistry(fakeRegistry.Addr())

					_, err := runner.Create(groot.CreateSpec{
						BaseImage: baseImageURL,
						ID:        "random-id-2",
						Mount:     true,
					})

					Expect(err).To(MatchError(ContainSubstring("layer is corrupted")))
					layerSnapshotPath := filepath.Join(StorePath, "volumes", "sha256:3355e23c079e9b35e4b48075147a7e7e1850b99e089af9a63eed3de235af98ca")
					Expect(layerSnapshotPath).ToNot(BeAnExistingFile())
				})
			})
		})

		Context("when the image has a version 1 manifest schema", func() {
			BeforeEach(func() {
				baseImageURL = "docker:///cfgarden/empty:schemaV1"
			})

			It("creates a root filesystem based on the image provided", func() {
				image, err := Runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     true,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(path.Join(image.Rootfs, "allo")).To(BeAnExistingFile())
				Expect(path.Join(image.Rootfs, "hello")).To(BeAnExistingFile())
			})
		})
	})

	Context("when a private registry is used", func() {
		var (
			fakeRegistry *testhelpers.FakeRegistry
		)

		BeforeEach(func() {
			dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
			Expect(err).NotTo(HaveOccurred())
			fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
			Expect(fakeRegistry.Start()).To(Succeed())

			baseImageURL = fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr())
		})

		AfterEach(func() {
			fakeRegistry.Stop()
		})

		It("fails to fetch the image", func() {
			_, err := Runner.Create(groot.CreateSpec{
				BaseImage: baseImageURL,
				ID:        "random-id",
				Mount:     true,
			})
			Expect(err).To(MatchError("This registry is insecure. To pull images from this registry, please use the --insecure-registry option."))
		})

		Context("when it's provided as a valid insecure registry", func() {
			It("should create a root filesystem based on the image provided by the private registry", func() {
				runner := Runner.WithInsecureRegistry(fakeRegistry.Addr())
				image, err := runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     true,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(path.Join(image.Rootfs, "hello")).To(BeARegularFile())
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
				})

				AfterEach(func() {
					Expect(os.RemoveAll(configDir)).To(Succeed())
				})

				It("creates a root filesystem based on the image provided by the private registry", func() {
					runner := Runner.WithConfig(configFilePath)
					image, err := runner.Create(groot.CreateSpec{
						BaseImage: baseImageURL,
						ID:        "random-id",
						Mount:     true,
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(path.Join(image.Rootfs, "hello")).To(BeARegularFile())
					Expect(fakeRegistry.RequestedBlobs()).To(HaveLen(3))
				})
			})

			Context("when config path is invalid", func() {
				It("returns a useful error", func() {
					runner := Runner.WithConfig("invalid-config-path")
					_, err := runner.Create(groot.CreateSpec{
						BaseImage: baseImageURL,
						ID:        "random-id",
						Mount:     true,
					})
					Expect(err).To(MatchError(ContainSubstring("invalid config path")))
				})
			})
		})
	})

	Context("when the image does not exist", func() {
		It("returns a useful error", func() {
			_, err := Runner.Create(groot.CreateSpec{
				BaseImage: "docker:///cfgaren/sorry-not-here",
				ID:        "random-id",
				Mount:     true,
			})
			Expect(err).To(MatchError(ContainSubstring("docker:///cfgaren/sorry-not-here does not exist or you do not have permissions to see it.")))
		})
	})

	Context("when the image has files that are not writable to their owner", func() {
		BeforeEach(func() {
			baseImageURL = "docker:///cfgarden/non-writable-file"
		})

		Context("when providing id mappings", func() {
			It("works", func() {
				image, err := Runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     true,
					UIDMappings: []groot.IDMappingSpec{
						{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
						{HostID: 100000, NamespaceID: 1, Size: 65000},
					},
					GIDMappings: []groot.IDMappingSpec{
						{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
						{HostID: 100000, NamespaceID: 1, Size: 65000},
					},
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(path.Join(image.Rootfs, "test", "hello")).To(BeARegularFile())
			})
		})
	})

	Context("when the image has folders that are not writable to their owner", func() {
		BeforeEach(func() {
			baseImageURL = "docker:///cfgarden/non-writable-folder"
		})

		Context("when providing id mappings", func() {
			It("works", func() {
				image, err := Runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     true,
					UIDMappings: []groot.IDMappingSpec{
						{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
						{HostID: 100000, NamespaceID: 1, Size: 65000},
					},
					GIDMappings: []groot.IDMappingSpec{
						{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
						{HostID: 100000, NamespaceID: 1, Size: 65000},
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(path.Join(image.Rootfs, "test", "hello")).To(BeARegularFile())
			})
		})
	})
})
