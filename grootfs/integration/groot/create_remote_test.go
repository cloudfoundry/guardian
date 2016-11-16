package groot_test

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
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Create with remote images", func() {
	var baseImageURL string

	Context("when using the default registry", func() {
		BeforeEach(func() {
			baseImageURL = "docker:///cfgarden/empty:v0.1.0"
		})

		It("creates a root filesystem based on the image provided", func() {
			image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImageURL, "random-id", 0)

			Expect(path.Join(image.RootFSPath, "hello")).To(BeARegularFile())
		})

		It("saves the image.json to the image folder", func() {
			image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImageURL, "random-id", 0)

			imageJsonPath := path.Join(image.Path, "image.json")
			Expect(imageJsonPath).To(BeARegularFile())

			imageJsonReader, err := os.Open(imageJsonPath)
			Expect(err).ToNot(HaveOccurred())
			var imageJson specsv1.Image
			Expect(json.NewDecoder(imageJsonReader).Decode(&imageJson)).To(Succeed())

			Expect(imageJson.Created).To(Equal("2016-08-03T16:50:55.797615406Z"))
			Expect(imageJson.RootFS.DiffIDs).To(Equal([]string{
				"sha256:3355e23c079e9b35e4b48075147a7e7e1850b99e089af9a63eed3de235af98ca",
			}))
		})

		Context("when the image has volumes", func() {
			BeforeEach(func() {
				baseImageURL = "docker:///cfgarden/with-volume"
			})

			It("creates the volume folders", func() {
				image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImageURL, "random-id", 0)
				volumeFolder := path.Join(image.RootFSPath, "foo")
				Expect(volumeFolder).To(BeADirectory())
			})
		})

		Context("when the image has opaque white outs", func() {
			BeforeEach(func() {
				baseImageURL = "docker:///cfgarden/opq-whiteout-busybox"
			})

			It("empties the folder contents but keeps the dir", func() {
				image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImageURL, "random-id", 0)

				whiteoutedDir := path.Join(image.RootFSPath, "var")
				Expect(whiteoutedDir).To(BeADirectory())
				contents, err := ioutil.ReadDir(whiteoutedDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(contents).To(BeEmpty())
			})
		})

		Context("when the image has files with the setuid on", func() {
			BeforeEach(func() {
				baseImageURL = "docker:///cfgarden/garden-busybox"
			})

			It("correctly applies the user bit", func() {
				image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImageURL, "random-id", 0)

				setuidFilePath := path.Join(image.RootFSPath, "bin", "busybox")
				stat, err := os.Stat(setuidFilePath)
				Expect(err).NotTo(HaveOccurred())

				Expect(stat.Mode() & os.ModeSetuid).To(Equal(os.ModeSetuid))
			})
		})

		Context("when image size exceeds disk quota", func() {
			BeforeEach(func() {
				baseImageURL = "docker:///cfgarden/empty:v0.1.1"
			})

			Context("when the image is not accounted for in the quota", func() {
				It("succeeds", func() {
					cmd := exec.Command(GrootFSBin, "--store", StorePath, "--drax-bin", DraxBin, "create", "--disk-limit-size-bytes", "10", "--exclude-image-from-quota", baseImageURL, "random-id")
					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess, 12*time.Second).Should(gexec.Exit(0))
				})
			})

			Context("when the image is accounted for in the quota", func() {
				It("returns an error", func() {
					cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", "--disk-limit-size-bytes", "10", baseImageURL, "random-id")

					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess, 12*time.Second).Should(gexec.Exit(1))

					Eventually(sess).Should(gbytes.Say("layers exceed disk quota"))
				})
			})
		})

		Describe("Unpacked layer caching", func() {
			It("caches the unpacked image in a subvolume with snapshots", func() {
				integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImageURL, "random-id", 0)

				layerSnapshotPath := filepath.Join(StorePath, CurrentUserID, "volumes", "sha256:3355e23c079e9b35e4b48075147a7e7e1850b99e089af9a63eed3de235af98ca")
				Expect(ioutil.WriteFile(layerSnapshotPath+"/injected-file", []byte{}, 0666)).To(Succeed())

				image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImageURL, "random-id-2", 0)
				Expect(path.Join(image.RootFSPath, "hello")).To(BeARegularFile())
				Expect(path.Join(image.RootFSPath, "injected-file")).To(BeARegularFile())
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
					cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", "--insecure-registry", fakeRegistry.Addr(), baseImageURL, "random-id-2")
					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess, 12*time.Second).Should(gexec.Exit(1))

					Eventually(sess).Should(gbytes.Say("layer is corrupted"))

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
				image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImageURL, "random-id", 0)
				Expect(path.Join(image.RootFSPath, "allo")).To(BeAnExistingFile())
				Expect(path.Join(image.RootFSPath, "hello")).To(BeAnExistingFile())
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
			_, err := integration.CreateImageWSpec(GrootFSBin, StorePath, DraxBin, groot.CreateSpec{
				ID:        "random-id",
				BaseImage: baseImageURL,
			})

			Eventually(err).Should(MatchError("This registry is insecure. To pull images from this registry, please use the --insecure-registry option."))
		})

		Context("when it's provided as a valid insecure registry", func() {
			It("should create a root filesystem based on the image provided by the private registry", func() {
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", "--insecure-registry", fakeRegistry.Addr(), baseImageURL, "random-id")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess, 12*time.Second).Should(gexec.Exit(0))

				rootFSPath := strings.TrimSpace(string(sess.Out.Contents())) + "/rootfs"
				Expect(path.Join(rootFSPath, "hello")).To(BeARegularFile())

				Expect(fakeRegistry.RequestedBlobs()).To(HaveLen(3))
			})
		})
	})

	Context("when the image does not exist", func() {
		It("returns a useful error", func() {
			_, err := integration.CreateImageWSpec(GrootFSBin, StorePath, DraxBin, groot.CreateSpec{
				ID:        "random-id",
				BaseImage: "docker:///cfgaren/sorry-not-here",
			})

			Eventually(err).Should(MatchError("docker:///cfgaren/sorry-not-here does not exist or you do not have permissions to see it."))
		})
	})
})
