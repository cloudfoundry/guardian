package remote_test

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"regexp"

	"code.cloudfoundry.org/grootfs/fetcher/remote"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Docker source", func() {
	var (
		trustedRegistries []string
		dockerSrc         *remote.DockerSource

		logger   lager.Logger
		imageURL *url.URL

		configBlob           string
		expectedLayersDigest []string
		expectedDiffIds      []string
	)

	BeforeEach(func() {
		trustedRegistries = []string{}

		configBlob = "sha256:217f3b4afdf698d639f854d9c6d640903a011413bc7e7bffeabe63c7ca7e4a7d"
		expectedLayersDigest = []string{
			"sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58",
			"sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76",
		}
		expectedDiffIds = []string{
			"sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
			"sha256:d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0",
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

			Expect(manifest.Config.Digest).To(Equal(configBlob))

			Expect(manifest.Layers).To(HaveLen(2))
			Expect(manifest.Layers[0].Digest).To(Equal(expectedLayersDigest[0]))
			Expect(manifest.Layers[1].Digest).To(Equal(expectedLayersDigest[1]))
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
			config, err := dockerSrc.Config(logger, imageURL, configBlob)
			Expect(err).NotTo(HaveOccurred())

			Expect(config.RootFS.DiffIDs).To(HaveLen(2))
			Expect(config.RootFS.DiffIDs[0]).To(Equal(expectedDiffIds[0]))
			Expect(config.RootFS.DiffIDs[1]).To(Equal(expectedDiffIds[1]))
		})

		Context("when the image or the config blob does not exist", func() {
			BeforeEach(func() {
				var err error
				imageURL, err = url.Parse("docker:///cfgarden/non-existing-image")
				Expect(err).NotTo(HaveOccurred())
			})

			It("retuns an error", func() {
				_, err := dockerSrc.Config(logger, imageURL, configBlob)
				Expect(err).To(MatchError(ContainSubstring("fetching config blob")))
			})
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = dockerSrc.Config(logger, imageURL, configBlob)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
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

			imageURL, err = url.Parse(fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", proxy.Addr()))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			proxy.Close()
		})

		It("fails to fetch the manifest", func() {
			_, err := dockerSrc.Manifest(logger, imageURL)
			Expect(err).To(MatchError(ContainSubstring("TLS validation of insecure registry failed")))
		})

		It("fails to fetch the Config", func() {
			_, err := dockerSrc.Config(logger, imageURL, configBlob)
			Expect(err).To(MatchError(ContainSubstring("TLS validation of insecure registry failed")))
		})

		Context("when the private registry is whitelisted", func() {
			BeforeEach(func() {
				trustedRegistries = []string{proxy.Addr()}
			})

			It("fetches the manifest", func() {
				manifest, err := dockerSrc.Manifest(logger, imageURL)
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.Layers).To(HaveLen(2))
				Expect(manifest.Layers[0].Digest).To(Equal(expectedLayersDigest[0]))
				Expect(manifest.Layers[1].Digest).To(Equal(expectedLayersDigest[1]))
			})

			It("fetches the config", func() {
				config, err := dockerSrc.Config(logger, imageURL, configBlob)
				Expect(err).NotTo(HaveOccurred())

				Expect(config.RootFS.DiffIDs).To(HaveLen(2))
				Expect(config.RootFS.DiffIDs[0]).To(Equal(expectedDiffIds[0]))
				Expect(config.RootFS.DiffIDs[1]).To(Equal(expectedDiffIds[1]))
			})
		})
	})

	Describe("StreamBlob", func() {
		It("steams a blob", func() {
			reader, _, err := dockerSrc.StreamBlob(logger, imageURL, expectedLayersDigest[0])
			Expect(err).NotTo(HaveOccurred())

			buffer := gbytes.NewBuffer()
			cmd := exec.Command("tar", "tv")
			cmd.Stdin = reader
			sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(buffer).Should(gbytes.Say("hello"))
			Eventually(sess).Should(gexec.Exit(0))
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, _, err = dockerSrc.StreamBlob(logger, imageURL, expectedLayersDigest[0])
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the blob does not exist", func() {
			It("returns an error", func() {
				_, _, err := dockerSrc.StreamBlob(logger, imageURL, "sha256:steamed-blob")
				Expect(err).To(MatchError(ContainSubstring("fetching blob 404")))
			})
		})

		Context("when the blob is not a gzip", func() {
			It("returns an error", func() {
				_, _, err := dockerSrc.StreamBlob(logger, imageURL, configBlob)
				Expect(err).To(MatchError(ContainSubstring("reading gzip")))
			})
		})
	})
})
