package source_test

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"
	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher/source"
	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher/source/sourcefakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/containers/image/v5/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Layer source: Docker", func() {
	var (
		layerSource source.LayerSource

		logger       *lagertest.TestLogger
		baseImageURL *url.URL

		configBlob    string
		blobPath      string
		layerInfos    []groot.LayerInfo
		systemContext types.SystemContext

		skipOCILayerValidation   bool
		skipImageQuotaValidation bool
		imageQuota               int64
		imageSourceCreator       source.ImageSourceCreator
	)

	BeforeEach(func() {
		blobPath = ""
		systemContext = types.SystemContext{
			DockerAuthConfig: &types.DockerAuthConfig{
				Username: RegistryUsername,
				Password: RegistryPassword,
			},
		}

		skipOCILayerValidation = false
		skipImageQuotaValidation = true
		imageSourceCreator = source.CreateImageSource
		imageQuota = 0

		configBlob = "sha256:217f3b4afdf698d639f854d9c6d640903a011413bc7e7bffeabe63c7ca7e4a7d"
		layerInfos = []groot.LayerInfo{
			{
				BlobID:    "sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58",
				DiffID:    "afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
				Size:      90,
			},
			{
				BlobID:    "sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76",
				DiffID:    "d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0",
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
				Size:      88,
			},
		}

		logger = lagertest.NewTestLogger("test-layer-source")
		var err error
		baseImageURL, err = url.Parse("docker:///cfgarden/empty:v0.1.1")
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		layerSource = source.NewLayerSource(systemContext, skipOCILayerValidation, skipImageQuotaValidation, imageQuota, baseImageURL, imageSourceCreator)
	})

	AfterEach(func() {
		if _, err := os.Stat(blobPath); err == nil {
			Expect(os.Remove(blobPath)).To(Succeed())
		}
	})

	Describe("Manifest", func() {
		It("fetches the manifest", func() {
			manifest, err := layerSource.Manifest(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.ConfigInfo().Digest.String()).To(Equal(configBlob))

			Expect(manifest.LayerInfos()).To(HaveLen(2))
			Expect(manifest.LayerInfos()[0].Digest.String()).To(Equal(layerInfos[0].BlobID))
			Expect(manifest.LayerInfos()[0].Size).To(Equal(layerInfos[0].Size))
			Expect(manifest.LayerInfos()[1].Digest.String()).To(Equal(layerInfos[1].BlobID))
			Expect(manifest.LayerInfos()[1].Size).To(Equal(layerInfos[1].Size))
		})

		Context("when the image schema version is 1", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker://cfgarden/empty:schemaV1")
				Expect(err).NotTo(HaveOccurred())
			})

			It("fetches the manifest", func() {
				manifest, err := layerSource.Manifest(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.ConfigInfo().Digest.String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.ConfigBlobID))

				Expect(manifest.LayerInfos()).To(HaveLen(3))
				Expect(manifest.LayerInfos()[0].Digest.String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.Layers[0].BlobID))
				Expect(manifest.LayerInfos()[0].Size).To(Equal(int64(-1)))
				Expect(manifest.LayerInfos()[1].Digest.String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.Layers[1].BlobID))
				Expect(manifest.LayerInfos()[1].Size).To(Equal(int64(-1)))
				Expect(manifest.LayerInfos()[2].Digest.String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.Layers[2].BlobID))
				Expect(manifest.LayerInfos()[2].Size).To(Equal(int64(-1)))
			})
		})

		Context("when the image is private", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:///cfgarden/private")
				Expect(err).NotTo(HaveOccurred())

				configBlob = "sha256:c2bf00eb303023869c676f91af930a12925c24d677999917e8d52c73fa10b73a"
				layerInfos[0].BlobID = "sha256:dabca1fccc91489bf9914945b95582f16d6090f423174641710083d6651db4a4"
				layerInfos[0].DiffID = "afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5"
				layerInfos[1].BlobID = "sha256:48ce60c2de08a424e10810c41ec2f00916cfd0f12333e96eb4363eb63723be87"
			})

			Context("when the correct credentials are provided", func() {
				It("fetches the manifest", func() {
					manifest, err := layerSource.Manifest(logger)
					Expect(err).NotTo(HaveOccurred())

					Expect(manifest.ConfigInfo().Digest.String()).To(Equal(configBlob))

					Expect(manifest.LayerInfos()).To(HaveLen(2))
					Expect(manifest.LayerInfos()[0].Digest.String()).To(Equal(layerInfos[0].BlobID))
					Expect(manifest.LayerInfos()[0].Size).To(Equal(layerInfos[0].Size))
					Expect(manifest.LayerInfos()[1].Digest.String()).To(Equal(layerInfos[1].BlobID))
					Expect(manifest.LayerInfos()[1].Size).To(Equal(layerInfos[1].Size))
				})
			})

			Context("when the registry returns a 401 when trying to get the auth token", func() {
				// We need a fake registry here because Dockerhub was rate limiting on multiple bad credential auth attempts
				var fakeRegistry *testhelpers.FakeRegistry

				BeforeEach(func() {
					dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
					Expect(err).NotTo(HaveOccurred())
					fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
					fakeRegistry.Start()
					fakeRegistry.ForceTokenAuthError()
					baseImageURL = integration.String2URL(fmt.Sprintf("docker://%s/doesnt-matter-because-fake-registry", fakeRegistry.Addr()))

					systemContext.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
				})

				AfterEach(func() {
					fakeRegistry.Stop()
				})

				It("returns an informative error", func() {
					_, err := layerSource.Manifest(logger)
					Expect(err).To(MatchError(ContainSubstring("unable to retrieve auth token")))
				})
			})
		})

		Context("when the image url is invalid", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())
			})
			It("returns an error", func() {
				_, err := layerSource.Manifest(logger)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the image does not exist", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:///cfgarden/non-existing-image")
				Expect(err).NotTo(HaveOccurred())

				systemContext.DockerAuthConfig.Username = ""
				systemContext.DockerAuthConfig.Password = ""
			})

			It("wraps the containers/image with a useful error", func() {
				_, err := layerSource.Manifest(logger)
				Expect(err.Error()).To(MatchRegexp("^fetching image reference"))
			})

			It("logs the original error message", func() {
				_, err := layerSource.Manifest(logger)
				Expect(err).To(HaveOccurred())

				Expect(logger).To(gbytes.Say("fetching-image-reference-failed"))
				Expect(logger).To(gbytes.Say("unauthorized: authentication required"))
			})
		})
	})

	Describe("Config", func() {
		It("fetches the config", func() {
			manifest, err := layerSource.Manifest(logger)
			Expect(err).NotTo(HaveOccurred())
			config, err := manifest.OCIConfig(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			Expect(config.RootFS.DiffIDs).To(HaveLen(2))
			Expect(config.RootFS.DiffIDs[0].Hex()).To(Equal(layerInfos[0].DiffID))
			Expect(config.RootFS.DiffIDs[1].Hex()).To(Equal(layerInfos[1].DiffID))
		})

		Context("when the image is private", func() {
			var manifest layer_fetcher.Manifest

			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:///cfgarden/private")
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				var err error
				manifest, err = layerSource.Manifest(logger)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the correct credentials are provided", func() {
				It("fetches the config", func() {
					config, err := manifest.OCIConfig(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					Expect(config.RootFS.DiffIDs).To(HaveLen(2))
					Expect(config.RootFS.DiffIDs[0].Hex()).To(Equal("780016ca8250bcbed0cbcf7b023c75550583de26629e135a1e31c0bf91fba296"))
					Expect(config.RootFS.DiffIDs[1].Hex()).To(Equal("56702ece901015f4f42dc82d1386c5ffc13625c008890d52548ff30dd142838b"))
				})
			})
		})

		Context("when the image url is invalid", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				_, err := layerSource.Manifest(logger)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the image schema version is 1", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker://cfgarden/empty:schemaV1")
				Expect(err).NotTo(HaveOccurred())
			})

			It("fetches the config", func() {
				manifest, err := layerSource.Manifest(logger)
				Expect(err).NotTo(HaveOccurred())
				config, err := manifest.OCIConfig(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				Expect(config.RootFS.DiffIDs).To(HaveLen(3))
				Expect(config.RootFS.DiffIDs[0].String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.Layers[0].DiffID))
				Expect(config.RootFS.DiffIDs[1].String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.Layers[1].DiffID))
				Expect(config.RootFS.DiffIDs[2].String()).To(Equal(testhelpers.SchemaV1EmptyBaseImage.Layers[2].DiffID))
			})
		})
	})

	Context("when registry communication fails temporarily", func() {
		var fakeRegistry *testhelpers.FakeRegistry

		BeforeEach(func() {
			dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
			Expect(err).NotTo(HaveOccurred())
			fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
			fakeRegistry.Start()

			systemContext.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
			baseImageURL, err = url.Parse(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			fakeRegistry.Stop()
		})

		It("retries fetching the manifest twice", func() {
			fakeRegistry.FailNextManifestRequests(2)

			_, err := layerSource.Manifest(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger.TestSink.LogMessages()).To(ContainElement("test-layer-source.fetching-image-manifest.attempt-get-image-1"))
			Expect(logger.TestSink.LogMessages()).To(ContainElement("test-layer-source.fetching-image-manifest.attempt-get-image-2"))
			Expect(logger.TestSink.LogMessages()).To(ContainElement("test-layer-source.fetching-image-manifest.attempt-get-image-3"))
			Expect(logger.TestSink.LogMessages()).To(ContainElement("test-layer-source.fetching-image-manifest.attempt-get-image-success"))
		})

		It("retries fetching a blob twice", func() {
			fakeRegistry.FailNextBlobRequests(2)

			var err error
			blobPath, _, err = layerSource.Blob(logger, layerInfos[0])
			Expect(err).NotTo(HaveOccurred())

			Expect(logger.TestSink.LogMessages()).To(
				ContainElement("test-layer-source.streaming-blob.attempt-get-blob-failed"))
		})

		It("retries fetching the config blob twice", func() {
			fakeRegistry.WhenGettingBlob(configBlob, 1, func(resp http.ResponseWriter, req *http.Request) {
				resp.WriteHeader(http.StatusTeapot)
				_, _ = resp.Write([]byte("null"))
				return
			})

			_, err := layerSource.Manifest(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeRegistry.RequestedBlobs()).To(Equal([]string{configBlob}), "config blob was not prefetched within the retry")

			Expect(logger.TestSink.LogMessages()).To(
				ContainElement("test-layer-source.fetching-image-manifest.fetching-image-config-failed"))
		})
	})

	Context("when a private registry is used", func() {
		var fakeRegistry *testhelpers.FakeRegistry

		BeforeEach(func() {
			dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
			Expect(err).NotTo(HaveOccurred())
			fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
			fakeRegistry.Start()

			baseImageURL, err = url.Parse(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			fakeRegistry.Stop()
		})

		It("fails to fetch the manifest", func() {
			_, err := layerSource.Manifest(logger)
			Expect(err).To(HaveOccurred())
		})

		Context("when the private registry is whitelisted", func() {
			BeforeEach(func() {
				systemContext.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
			})

			It("fetches the manifest", func() {
				manifest, err := layerSource.Manifest(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.LayerInfos()).To(HaveLen(2))
				Expect(manifest.LayerInfos()[0].Digest.String()).To(Equal(layerInfos[0].BlobID))
				Expect(manifest.LayerInfos()[0].Size).To(Equal(layerInfos[0].Size))
				Expect(manifest.LayerInfos()[1].Digest.String()).To(Equal(layerInfos[1].BlobID))
				Expect(manifest.LayerInfos()[1].Size).To(Equal(layerInfos[1].Size))
			})

			It("fetches the config", func() {
				manifest, err := layerSource.Manifest(logger)
				Expect(err).NotTo(HaveOccurred())

				config, err := manifest.OCIConfig(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				Expect(config.RootFS.DiffIDs).To(HaveLen(2))
				Expect(config.RootFS.DiffIDs[0].Hex()).To(Equal(layerInfos[0].DiffID))
				Expect(config.RootFS.DiffIDs[1].Hex()).To(Equal(layerInfos[1].DiffID))
			})

			It("downloads and uncompresses the blob", func() {
				var (
					size int64
					err  error
				)

				blobPath, size, err = layerSource.Blob(logger, layerInfos[0])
				Expect(err).NotTo(HaveOccurred())

				blobReader, err := os.Open(blobPath)
				Expect(err).NotTo(HaveOccurred())

				buffer := gbytes.NewBuffer()
				cmd := exec.Command("tar", "tv")
				cmd.Stdin = blobReader
				sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Expect(size).To(Equal(int64(90)))

				Eventually(buffer, "2s").Should(gbytes.Say("hello"))
				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Context("when using private images", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:///cfgarden/private")
				Expect(err).NotTo(HaveOccurred())

				layerInfos[0].BlobID = "sha256:dabca1fccc91489bf9914945b95582f16d6090f423174641710083d6651db4a4"
				layerInfos[0].DiffID = "780016ca8250bcbed0cbcf7b023c75550583de26629e135a1e31c0bf91fba296"
				layerInfos[1].BlobID = "sha256:48ce60c2de08a424e10810c41ec2f00916cfd0f12333e96eb4363eb63723be87"
				layerInfos[1].DiffID = "56702ece901015f4f42dc82d1386c5ffc13625c008890d52548ff30dd142838b"
			})

			It("fetches the manifest", func() {
				manifest, err := layerSource.Manifest(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.LayerInfos()).To(HaveLen(2))
				Expect(manifest.LayerInfos()[0].Digest.String()).To(Equal(layerInfos[0].BlobID))
				Expect(manifest.LayerInfos()[0].Size).To(Equal(layerInfos[0].Size))
				Expect(manifest.LayerInfos()[1].Digest.String()).To(Equal(layerInfos[1].BlobID))
				Expect(manifest.LayerInfos()[1].Size).To(Equal(layerInfos[1].Size))
			})

			It("fetches the config", func() {
				manifest, err := layerSource.Manifest(logger)
				Expect(err).NotTo(HaveOccurred())

				config, err := manifest.OCIConfig(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				Expect(config.RootFS.DiffIDs).To(HaveLen(2))
				Expect(config.RootFS.DiffIDs[0].Hex()).To(Equal(layerInfos[0].DiffID))
				Expect(config.RootFS.DiffIDs[1].Hex()).To(Equal(layerInfos[1].DiffID))
			})

			It("downloads and uncompresses the blob", func() {
				var (
					size int64
					err  error
				)

				blobPath, size, err = layerSource.Blob(logger, layerInfos[0])
				Expect(err).NotTo(HaveOccurred())

				blobReader, err := os.Open(blobPath)
				Expect(err).NotTo(HaveOccurred())

				buffer := gbytes.NewBuffer()
				cmd := exec.Command("tar", "tv")
				cmd.Stdin = blobReader
				sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Expect(size).To(Equal(int64(90)))

				Eventually(buffer).Should(gbytes.Say("hello"))
				Eventually(sess).Should(gexec.Exit(0))
			})
		})
	})

	Describe("Blob", func() {
		var (
			blobSize  int64
			blobErr   error
			layerInfo groot.LayerInfo
		)

		BeforeEach(func() {
			layerInfo = layerInfos[0]
		})

		JustBeforeEach(func() {
			blobPath, blobSize, blobErr = layerSource.Blob(logger, layerInfo)
		})

		It("downloads and uncompresses the blob", func() {
			Expect(blobErr).NotTo(HaveOccurred())

			blobReader, err := os.Open(blobPath)
			Expect(err).NotTo(HaveOccurred())
			defer blobReader.Close()

			cmd := exec.Command("tar", "tv")
			cmd.Stdin = blobReader
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(blobSize).To(Equal(int64(90)))

			Eventually(sess.Out).Should(gbytes.Say("hello"))
			Eventually(sess).Should(gexec.Exit(0))
		})

		Context("when the media type doesn't match the blob", func() {
			var fakeRegistry *testhelpers.FakeRegistry

			BeforeEach(func() {
				dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
				Expect(err).NotTo(HaveOccurred())
				fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)

				fakeRegistry.WhenGettingBlob(layerInfos[0].BlobID, 1, func(rw http.ResponseWriter, req *http.Request) {
					io.WriteString(rw, "bad-blob")
				})

				fakeRegistry.Start()

				baseImageURL, err = url.Parse(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
				Expect(err).NotTo(HaveOccurred())

				systemContext.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue

				layerInfo.MediaType = "gzip"
			})

			AfterEach(func() {
				fakeRegistry.Stop()
			})

			It("returns an error", func() {
				Expect(blobErr).To(MatchError(ContainSubstring("layer size is different from the value in the manifest")))
			})
		})

		Context("when the image is private", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:///cfgarden/private")
				Expect(err).NotTo(HaveOccurred())

				layerInfo = groot.LayerInfo{
					BlobID:    "sha256:dabca1fccc91489bf9914945b95582f16d6090f423174641710083d6651db4a4",
					DiffID:    "780016ca8250bcbed0cbcf7b023c75550583de26629e135a1e31c0bf91fba296",
					MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
					Size:      90,
				}
			})

			Context("when the correct credentials are provided", func() {
				It("fetches the config", func() {
					Expect(blobErr).NotTo(HaveOccurred())

					blobReader, err := os.Open(blobPath)
					Expect(err).NotTo(HaveOccurred())

					buffer := gbytes.NewBuffer()
					cmd := exec.Command("tar", "tv")
					cmd.Stdin = blobReader
					sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Expect(blobSize).To(Equal(int64(90)))

					Eventually(buffer, 5*time.Second).Should(gbytes.Say("hello"))
					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("when invalid credentials are provided", func() {
				// We need a fake registry here because Dockerhub was rate limiting on multiple bad credential auth attempts
				var fakeRegistry *testhelpers.FakeRegistry

				BeforeEach(func() {
					dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
					Expect(err).NotTo(HaveOccurred())
					fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
					fakeRegistry.Start()
					fakeRegistry.ForceTokenAuthError()
					baseImageURL = integration.String2URL(fmt.Sprintf("docker://%s/doesnt-matter-because-fake-registry", fakeRegistry.Addr()))

					systemContext.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
				})

				AfterEach(func() {
					fakeRegistry.Stop()
				})

				It("retuns an error", func() {
					Expect(blobErr).To(MatchError(ContainSubstring("unable to retrieve auth token")))
				})
			})
		})

		Context("when the image url is invalid", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				Expect(blobErr).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the blob does not exist", func() {
			BeforeEach(func() {
				layerInfo = groot.LayerInfo{BlobID: "sha256:3a50a9ff45117c33606ba54f4a7f55cebbdd2e96a06ab48e7e981a02ff1fd665"}
			})

			It("returns an error", func() {
				Expect(blobErr).To(MatchError(And(ContainSubstring("fetching blob"), ContainSubstring("404"))))
			})
		})

		Context("when the blob is corrupted", func() {
			var fakeRegistry *testhelpers.FakeRegistry

			BeforeEach(func() {
				layerInfo = layerInfos[1]
				dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
				Expect(err).NotTo(HaveOccurred())
				fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
				fakeRegistry.WhenGettingBlob(layerInfo.BlobID, 1, func(rw http.ResponseWriter, req *http.Request) {
					gzipWriter := gzip.NewWriter(rw)
					io.WriteString(gzipWriter, "bad-blob")
					gzipWriter.Close()
				})
				fakeRegistry.Start()

				baseImageURL, err = url.Parse(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
				Expect(err).NotTo(HaveOccurred())

				systemContext.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
			})

			AfterEach(func() {
				fakeRegistry.Stop()
			})

			It("returns an error", func() {
				Expect(blobErr).To(MatchError(ContainSubstring("layer size is different from the value in the manifest")))
			})

			Context("when a devious hacker tries to set skipOCILayerValidation to true", func() {
				BeforeEach(func() {
					skipOCILayerValidation = true
				})

				It("returns an error", func() {
					Expect(blobErr).To(MatchError(ContainSubstring("layerID digest mismatch")))
				})
			})
		})

		Context("when the blob doesn't match the diffID", func() {
			BeforeEach(func() {
				layerInfo = layerInfos[1]
			})
			BeforeEach(func() {
				layerInfo.DiffID = "0000000000000000000000000000000000000000000000000000000000000000"
			})

			It("returns an error", func() {
				Expect(blobErr).To(MatchError(ContainSubstring("diffID digest mismatch")))
			})
		})

		Context("when image quota validation is not skipped", func() {
			BeforeEach(func() {
				skipImageQuotaValidation = false
			})

			Context("when the uncompressed layer size is bigger that the quota", func() {
				BeforeEach(func() {
					imageQuota = 1
				})

				It("returns quota exceeded error", func() {
					Expect(blobErr).To(MatchError(ContainSubstring("uncompressed layer size exceeds quota")))
				})
			})
		})

		Context("when the docker registry does not report blob size", func() {
			var blob io.ReadCloser
			BeforeEach(func() {
				fakeImageSourceCreator := new(sourcefakes.FakeImageSourceCreator)
				imageSourceCreator = fakeImageSourceCreator.Spy
				imageSource := new(sourcefakes.FakeImageSource)

				layerInfo.BlobID = "sha256:b454a485b4601f27bc4bd08d6136a69244ac778423caf37deb65e4749f691f84"
				layerInfo.DiffID = "3d16a9d9679dba04b90edeeca13dfaa986a268a7e0f4764250bacc2bed236b73"
				layerInfo.Size = 1145393

				var err error
				blob, err = os.Open(filepath.FromSlash("../../../integration/assets/remote-layers/garden-busybox-remote/b454a485b4601f27bc4bd08d6136a69244ac778423caf37deb65e4749f691f84"))
				Expect(err).NotTo(HaveOccurred())
				imageSource.GetBlobReturns(blob, -1, nil)
				fakeImageSourceCreator.Returns(imageSource, nil)
			})

			AfterEach(func() {
				blob.Close()
			})

			It("does not fail", func() {
				Expect(blobErr).NotTo(HaveOccurred())
			})

			It("returns the actual blob size", func() {
				Expect(blobSize).To(Equal(int64(1145393)))
			})

			Context("when the actual layer size does not match the manifest layer size", func() {
				BeforeEach(func() {
					layerInfo.Size = 123
				})

				It("returns an error", func() {
					Expect(blobErr).To(MatchError(ContainSubstring("layer size is different from the value in the manifest")))
				})
			})
		})
	})

	Describe("Close", func() {
		It("can close prior any interactions", func() {
			Expect(layerSource.Close()).To(Succeed())
		})
	})
})
