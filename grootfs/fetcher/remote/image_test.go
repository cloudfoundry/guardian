package remote_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"time"

	"code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/grootfs/fetcher/fetcherfakes"
	"code.cloudfoundry.org/grootfs/fetcher/remote"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/containers/image/docker"
	"github.com/containers/image/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Image", func() {
	var (
		ref               types.ImageReference
		fakeCacheDriver   *fetcherfakes.FakeCacheDriver
		image             *remote.ContainersImage
		logger            lager.Logger
		trustedRegistries []string
	)

	BeforeEach(func() {
		var err error
		ref, err = docker.ParseReference("//cfgarden/empty:v0.1.1")
		Expect(err).NotTo(HaveOccurred())

		fakeCacheDriver = new(fetcherfakes.FakeCacheDriver)
		fakeCacheDriver.BlobStub = func(logger lager.Logger, id string, streamBlob fetcher.StreamBlob) (io.ReadCloser, error) {
			return streamBlob(logger)
		}

		trustedRegistries = []string{}
		logger = lagertest.NewTestLogger("test-image")
	})

	JustBeforeEach(func() {
		image = remote.NewContainersImage(ref, fakeCacheDriver, trustedRegistries)
	})

	Describe("Manifest", func() {
		It("fetches the manifest", func() {
			manifest, err := image.Manifest(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.Layers).To(HaveLen(2))
			Expect(manifest.Layers[0].Digest).To(Equal("sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58"))
			Expect(manifest.Layers[1].Digest).To(Equal("sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76"))
		})

		Context("manifest caching in memory", func() {
			It("makes the second call a thousand times faster", func() {
				firstManifestCall := time.Now()
				_, err := image.Manifest(logger)
				Expect(err).NotTo(HaveOccurred())
				firstManifestCallElapsed := time.Since(firstManifestCall).Nanoseconds()

				secondManifestCall := time.Now()
				_, err = image.Manifest(logger)
				Expect(err).NotTo(HaveOccurred())
				secondManifestCallElapsed := time.Since(secondManifestCall).Nanoseconds()

				Expect(secondManifestCallElapsed).To(BeNumerically("<", firstManifestCallElapsed/1000))
			})
		})

		Context("when the image does not exist", func() {
			BeforeEach(func() {
				var err error
				ref, err = docker.ParseReference("//cfgarden/non-existing-image")
				Expect(err).NotTo(HaveOccurred())
			})

			It("wraps the containers/image with an useful error", func() {
				_, err := image.Manifest(logger)
				Expect(err).To(MatchError(ContainSubstring("image does not exist or you do not have permissions to see it")))
			})

			It("logs the original error message", func() {
				_, err := image.Manifest(logger)
				Expect(err).To(HaveOccurred())

				Expect(logger).To(gbytes.Say("fetching-manifest-failed"))
				Expect(logger).To(gbytes.Say("error fetching manifest: status code:"))
			})
		})
	})

	Describe("Config", func() {
		It("fetches the config", func() {
			config, err := image.Config(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(config.RootFS.DiffIDs).To(HaveLen(2))
			Expect(config.RootFS.DiffIDs[0]).To(Equal("sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5"))
			Expect(config.RootFS.DiffIDs[1]).To(Equal("sha256:d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0"))
		})

		It("uses the cache driver", func() {
			_, err := image.Config(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCacheDriver.BlobCallCount()).To(Equal(1))
		})

		Context("when the cache driver fails", func() {
			BeforeEach(func() {
				fakeCacheDriver.BlobReturns(nil, errors.New("unable to read/write cache"))
			})

			It("returns an error", func() {
				_, err := image.Config(logger)
				Expect(err).To(MatchError(ContainSubstring("unable to read/write cache")))
			})
		})

		Context("when the image does not exist", func() {
			BeforeEach(func() {
				var err error
				ref, err = docker.ParseReference("//cfgarden/non-existing-image")
				Expect(err).NotTo(HaveOccurred())
			})

			It("retuns an error", func() {
				_, err := image.Config(logger)
				Expect(err).To(MatchError(ContainSubstring("fetching manifest")))
			})
		})
	})

	Context("when a private registry is used", func() {
		var proxy *ghttp.Server

		BeforeEach(func() {
			dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
			Expect(err).NotTo(HaveOccurred())

			revProxy := httputil.NewSingleHostReverseProxy(dockerHubUrl)

			// Dockerhub returns 503 if the host is set to localhost
			// as it happens with the reverse proxy
			oldDirector := revProxy.Director
			revProxy.Director = func(req *http.Request) {
				oldDirector(req)
				req.Host = "registry-1.docker.io"
			}

			proxy = ghttp.NewTLSServer()
			ourRegexp, err := regexp.Compile(`.*`)
			Expect(err).NotTo(HaveOccurred())
			proxy.RouteToHandler("GET", ourRegexp, revProxy.ServeHTTP)

			ref, err = docker.ParseReference(fmt.Sprintf("//%s/cfgarden/empty:v0.1.1", proxy.Addr()))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			proxy.Close()
		})

		It("fails to fetch the manifest", func() {
			_, err := image.Manifest(logger)
			Expect(err).To(MatchError(ContainSubstring("TLS validation of insecure registry failed")))
		})

		It("fails to fetch the Config", func() {
			_, err := image.Config(logger)
			Expect(err).To(MatchError(ContainSubstring("TLS validation of insecure registry failed")))
		})

		Context("when the private registry is whitelisted", func() {
			BeforeEach(func() {
				trustedRegistries = []string{proxy.Addr()}
			})

			It("fetches the manifest", func() {
				manifest, err := image.Manifest(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.Layers).To(HaveLen(2))
				Expect(manifest.Layers[0].Digest).To(Equal("sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58"))
				Expect(manifest.Layers[1].Digest).To(Equal("sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76"))
			})

			It("fetches the config", func() {
				config, err := image.Config(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(config.RootFS.DiffIDs).To(HaveLen(2))
				Expect(config.RootFS.DiffIDs[0]).To(Equal("sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5"))
				Expect(config.RootFS.DiffIDs[1]).To(Equal("sha256:d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0"))
			})
		})
	})
})
