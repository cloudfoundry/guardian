package fetcher_test

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"

	clonerpkg "code.cloudfoundry.org/grootfs/cloner"
	fetcherpkg "code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fetcher", func() {
	var (
		fetcher         *fetcherpkg.Fetcher
		logger          *lagertest.TestLogger
		expectedDigests []clonerpkg.LayerDigest
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-fetcher")
		fetcher = fetcherpkg.NewFetcher("/cache-path")

		expectedDigests = []clonerpkg.LayerDigest{
			clonerpkg.LayerDigest{
				BlobID:        "sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58",
				DiffID:        "sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
				ChainID:       "sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
				ParentChainID: "",
			},
			clonerpkg.LayerDigest{
				BlobID:        "sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76",
				DiffID:        "sha256:d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0",
				ChainID:       "sha256:9242945d3c9c7cf5f127f9352fea38b1d3efe62ee76e25f70a3e6db63a14c233",
				ParentChainID: "sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
			},
		}
	})

	Describe("LayersDigest", func() {
		It("returns the correct list of layer digests", func() {
			imageURL, err := url.Parse("docker:///cfgarden/empty:v0.1.1")
			Expect(err).NotTo(HaveOccurred())

			digests, err := fetcher.LayersDigest(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(digests).To(Equal(expectedDigests))
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = fetcher.LayersDigest(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the image doesn't exist", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker:///cfgarden/empty:ImNotGroot")
				Expect(err).NotTo(HaveOccurred())

				digests, err := fetcher.LayersDigest(logger, imageURL)
				Expect(digests).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("inspecting image")))
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
			})

			AfterEach(func() {
				proxy.Close()
			})

			It("should create a root filesystem based on the image provided by the private registry", func() {
				imageURL, err := url.Parse(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", proxy.Addr()))
				Expect(err).NotTo(HaveOccurred())

				digests, err := fetcher.LayersDigest(logger, imageURL)
				Expect(err).NotTo(HaveOccurred())
				Expect(digests).To(Equal(expectedDigests))
			})
		})

		Context("when a private registry that doesn't exist is used", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker://my-awesome-non-existing-registry.com/cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = fetcher.LayersDigest(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("inspecting image")))
			})
		})
	})

	Describe("Streamer", func() {
		It("returns a streamer", func() {
			imageURL, err := url.Parse("docker:///cfgarden/empty:v0.1.0")
			Expect(err).NotTo(HaveOccurred())

			streamer, err := fetcher.Streamer(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(streamer).NotTo(BeNil())
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = fetcher.Streamer(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})
	})
})
