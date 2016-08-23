package fetcher_test

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"

	fetcherpkg "code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/containers/image/docker"
	"github.com/containers/image/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Image", func() {
	var (
		ref    types.ImageReference
		image  *fetcherpkg.ContainersImage
		logger lager.Logger
	)

	BeforeEach(func() {
		var err error
		ref, err = docker.ParseReference("//cfgarden/empty:v0.1.1")
		Expect(err).NotTo(HaveOccurred())

		logger = lagertest.NewTestLogger("test-image")
	})

	JustBeforeEach(func() {
		image = fetcherpkg.NewContainersImage(ref)
	})

	Describe("Manifest", func() {
		It("fetches the manifest", func() {
			manifest, err := image.Manifest(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.Layers).To(HaveLen(2))
			Expect(manifest.Layers[0].Digest).To(Equal("sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58"))
			Expect(manifest.Layers[1].Digest).To(Equal("sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76"))
		})

		Context("when the image does not exist", func() {
			BeforeEach(func() {
				var err error
				ref, err = docker.ParseReference("//cfgarden/non-existing-image")
				Expect(err).NotTo(HaveOccurred())
			})

			It("retuns an error", func() {
				_, err := image.Manifest(logger)
				Expect(err).To(MatchError(ContainSubstring("fetching manifest")))
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
