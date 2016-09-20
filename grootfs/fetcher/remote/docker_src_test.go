package remote_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/grootfs/fetcher/remote"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Docker source", func() {
	var (
		trustedRegistries []string
		dockerSrc         *remote.DockerSource

		logger   lager.Logger
		imageURL *url.URL

		configBlob           string
		expectedLayersDigest []remote.Layer
		expectedDiffIds      []string
		manifest             remote.Manifest
	)

	BeforeEach(func() {
		trustedRegistries = []string{}

		configBlob = "sha256:217f3b4afdf698d639f854d9c6d640903a011413bc7e7bffeabe63c7ca7e4a7d"
		expectedLayersDigest = []remote.Layer{
			remote.Layer{
				BlobID: "sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58",
				Size:   90,
			},
			remote.Layer{
				BlobID: "sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76",
				Size:   88,
			},
		}
		expectedDiffIds = []string{
			"sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
			"sha256:d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0",
		}

		manifest = remote.Manifest{
			ConfigCacheKey: configBlob,
			SchemaVersion:  2,
		}

		logger = lagertest.NewTestLogger("test-docker-source")
		var err error
		imageURL, err = url.Parse("docker:///cfgarden/empty:v0.1.1")
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		dockerSrc = remote.NewDockerSource(trustedRegistries)
	})

	Describe("Manifest", func() {
		It("fetches the manifest", func() {
			manifest, err := dockerSrc.Manifest(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.ConfigCacheKey).To(Equal(configBlob))

			Expect(manifest.Layers).To(HaveLen(2))
			Expect(manifest.Layers[0]).To(Equal(expectedLayersDigest[0]))
			Expect(manifest.Layers[1]).To(Equal(expectedLayersDigest[1]))
		})

		Context("when the image schema version is 1", func() {
			BeforeEach(func() {
				var err error
				imageURL, err = url.Parse("docker:///nginx:1.9")
				Expect(err).NotTo(HaveOccurred())
			})

			It("fetches the manifest", func() {
				manifest, err := dockerSrc.Manifest(logger, imageURL)
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.Layers).To(HaveLen(8))
				Expect(manifest.Layers[0]).To(Equal(remote.Layer{BlobID: "sha256:51f5c6a04d83efd2d45c5fd59537218924bc46705e3de6ffc8bc07b51481610b"}))
				Expect(manifest.Layers[1]).To(Equal(remote.Layer{BlobID: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"}))
				Expect(manifest.Layers[2]).To(Equal(remote.Layer{BlobID: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"}))
				Expect(manifest.Layers[3]).To(Equal(remote.Layer{BlobID: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"}))
				Expect(manifest.Layers[4]).To(Equal(remote.Layer{BlobID: "sha256:640c8f3d0eb2b84205cc43e312914c4ae464d433089ee1c95042b893eb7af09b"}))
				Expect(manifest.Layers[5]).To(Equal(remote.Layer{BlobID: "sha256:a4335300aa893de13a747fee474cd982c62539fd8e20e9b5eb21125996140b8a"}))
				Expect(manifest.Layers[6]).To(Equal(remote.Layer{BlobID: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"}))
				Expect(manifest.Layers[7]).To(Equal(remote.Layer{BlobID: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"}))

				Expect(manifest.ConfigCacheKey).To(Equal("sha256:f0f2e4b0f880c47ef68d8bca346ced37d32712b671412704524ac4162fbf944d"))
			})
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = dockerSrc.Manifest(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the image does not exist", func() {
			BeforeEach(func() {
				var err error
				imageURL, err = url.Parse("docker:///cfgarden/non-existing-image")
				Expect(err).NotTo(HaveOccurred())
			})

			It("wraps the containers/image with an useful error", func() {
				_, err := dockerSrc.Manifest(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("image does not exist or you do not have permissions to see it")))
			})

			It("logs the original error message", func() {
				_, err := dockerSrc.Manifest(logger, imageURL)
				Expect(err).To(HaveOccurred())

				Expect(logger).To(gbytes.Say("fetching-manifest-failed"))
				Expect(logger).To(gbytes.Say("error fetching manifest: status code:"))
			})
		})
	})

	Describe("Config", func() {
		It("fetches the config", func() {
			config, err := dockerSrc.Config(logger, imageURL, manifest)
			Expect(err).NotTo(HaveOccurred())

			Expect(config.RootFS.DiffIDs).To(HaveLen(2))
			Expect(config.RootFS.DiffIDs[0]).To(Equal(expectedDiffIds[0]))
			Expect(config.RootFS.DiffIDs[1]).To(Equal(expectedDiffIds[1]))
		})

		Context("when schema version is not supported", func() {
			BeforeEach(func() {
				manifest = remote.Manifest{
					SchemaVersion: 666,
				}
			})

			It("returns an error", func() {
				_, err := dockerSrc.Config(logger, imageURL, manifest)
				Expect(err).To(MatchError(ContainSubstring("schema version not supported")))
			})
		})

		Context("when the image or the config blob does not exist", func() {
			BeforeEach(func() {
				var err error
				imageURL, err = url.Parse("docker:///cfgarden/non-existing-image")
				Expect(err).NotTo(HaveOccurred())
			})

			It("retuns an error", func() {
				_, err := dockerSrc.Config(logger, imageURL, manifest)
				Expect(err).To(MatchError(ContainSubstring("fetching config blob")))
			})
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = dockerSrc.Config(logger, imageURL, manifest)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the image schema version is 1", func() {
			BeforeEach(func() {
				var err error
				imageURL, err = url.Parse("docker:///nginx:1.9")
				Expect(err).NotTo(HaveOccurred())
			})

			It("fetches the config", func() {
				manifest, err := dockerSrc.Manifest(logger, imageURL)
				Expect(err).NotTo(HaveOccurred())
				config, err := dockerSrc.Config(logger, imageURL, manifest)
				Expect(err).NotTo(HaveOccurred())

				Expect(config.RootFS.DiffIDs).To(HaveLen(8))
				Expect(config.RootFS.DiffIDs[0]).To(Equal("sha256:ab998debe217fc9749dba7168a9e4910c1e23f839fb902358cee96c3b7f4585c"))
				Expect(config.RootFS.DiffIDs[1]).To(Equal("sha256:c7fb0a077d24adf502a849eb20caebf5e5485bbceff66ecfe6d20221a57d8cd0"))
				Expect(config.RootFS.DiffIDs[2]).To(Equal("sha256:1f3613e168c1d0aaa5a5e9990eddba507b0ecd97fc47545fa09e19b78229684c"))
				Expect(config.RootFS.DiffIDs[3]).To(Equal("sha256:6f88661d681e0263f332fee3c04e3f88a3dda9f8eebf6a2f93ec4232719488e2"))
				Expect(config.RootFS.DiffIDs[4]).To(Equal("sha256:251bbadf08c36fdae6d4907da26fcc1cbe71c7c8f0e0eb094b0115f29af372fa"))
				Expect(config.RootFS.DiffIDs[5]).To(Equal("sha256:cc90a59ac496494827ce95c26257991d56bbb8b38556399985949b896bef7801"))
				Expect(config.RootFS.DiffIDs[6]).To(Equal("sha256:aa527287a51f0178662d50479697b53893b65fee9383af889ece937fd02c7c56"))
				Expect(config.RootFS.DiffIDs[7]).To(Equal("sha256:0e181a348ded1545ce2a2e84cf84017283315a9ec573959b0e3638ca95e36809"))
			})

			Context("when the manifest's v1Compatibility is empty", func() {
				BeforeEach(func() {
					manifest = remote.Manifest{
						SchemaVersion:   1,
						V1Compatibility: []string{},
					}
				})

				It("returns an error", func() {
					_, err := dockerSrc.Config(logger, imageURL, manifest)
					Expect(err).To(MatchError(ContainSubstring("V1Compatibility is empty for the manifest")))
				})
			})

			Context("when the manifest history is corrupted", func() {
				BeforeEach(func() {
					manifest = remote.Manifest{
						SchemaVersion:   1,
						V1Compatibility: []string{"not-json"},
					}
				})

				It("returns an error", func() {
					_, err := dockerSrc.Config(logger, imageURL, manifest)
					Expect(err).To(MatchError(ContainSubstring("parsing manifest V1Compatibility:")))
				})
			})
		})
	})

	Context("when a private registry is used", func() {
		var fakeRegistry *testhelpers.FakeRegistry

		BeforeEach(func() {
			dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
			Expect(err).NotTo(HaveOccurred())
			fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
			Expect(fakeRegistry.Start()).To(Succeed())

			imageURL, err = url.Parse(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			fakeRegistry.Stop()
		})

		It("fails to fetch the manifest", func() {
			_, err := dockerSrc.Manifest(logger, imageURL)
			Expect(err).To(MatchError(ContainSubstring("This registry is insecure. To pull images from this registry, please use the --insecure-registry option")))
		})

		It("fails to fetch the Config", func() {
			_, err := dockerSrc.Config(logger, imageURL, manifest)
			Expect(err).To(MatchError(ContainSubstring("This registry is insecure. To pull images from this registry, please use the --insecure-registry option")))
		})

		Context("when the private registry is whitelisted", func() {
			BeforeEach(func() {
				trustedRegistries = []string{fakeRegistry.Addr()}
			})

			It("fetches the manifest", func() {
				manifest, err := dockerSrc.Manifest(logger, imageURL)
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.Layers).To(HaveLen(2))
				Expect(manifest.Layers[0]).To(Equal(expectedLayersDigest[0]))
				Expect(manifest.Layers[1]).To(Equal(expectedLayersDigest[1]))
			})

			It("fetches the config", func() {
				config, err := dockerSrc.Config(logger, imageURL, manifest)
				Expect(err).NotTo(HaveOccurred())

				Expect(config.RootFS.DiffIDs).To(HaveLen(2))
				Expect(config.RootFS.DiffIDs[0]).To(Equal(expectedDiffIds[0]))
				Expect(config.RootFS.DiffIDs[1]).To(Equal(expectedDiffIds[1]))
			})
		})
	})

	Describe("Blob", func() {
		It("downloads a blob", func() {
			blobContents, size, err := dockerSrc.Blob(logger, imageURL, expectedLayersDigest[0].BlobID)
			Expect(err).NotTo(HaveOccurred())

			buffer := gbytes.NewBuffer()
			cmd := exec.Command("tar", "tzv")
			cmd.Stdin = bytes.NewBuffer(blobContents)
			sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(int64(90)))

			Eventually(buffer).Should(gbytes.Say("hello"))
			Eventually(sess).Should(gexec.Exit(0))
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, _, err = dockerSrc.Blob(logger, imageURL, expectedLayersDigest[0].BlobID)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the blob does not exist", func() {
			It("returns an error", func() {
				_, _, err := dockerSrc.Blob(logger, imageURL, "sha256:steamed-blob")
				Expect(err).To(MatchError(ContainSubstring("fetching blob 404")))
			})
		})

		Context("when the blob is corrupted", func() {
			var fakeRegistry *testhelpers.FakeRegistry

			BeforeEach(func() {
				dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
				Expect(err).NotTo(HaveOccurred())
				fakeRegistry = testhelpers.NewFakeRegistry(dockerHubUrl)
				layerDigest := strings.Split(expectedLayersDigest[1].BlobID, ":")[1]
				fakeRegistry.WhenGettingBlob(layerDigest, 2, func(rw http.ResponseWriter, req *http.Request) {
					rw.Write([]byte("bad-blob"))
				})
				Expect(fakeRegistry.Start()).To(Succeed())

				imageURL, err = url.Parse(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", fakeRegistry.Addr()))
				Expect(err).NotTo(HaveOccurred())

				trustedRegistries = []string{fakeRegistry.Addr()}
			})

			AfterEach(func() {
				fakeRegistry.Stop()
			})

			It("returns an error", func() {
				_, _, err := dockerSrc.Blob(logger, imageURL, expectedLayersDigest[1].BlobID)
				Expect(err).To(MatchError(ContainSubstring("invalid checksum: layer is corrupted")))
			})
		})
	})
})
