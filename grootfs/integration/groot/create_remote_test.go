package groot_test

import (
	"archive/tar"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Create with remote images", func() {
	var imageURL string

	Context("when using the default registry", func() {
		BeforeEach(func() {
			imageURL = "docker:///cfgarden/empty:v0.1.0"
		})

		It("creates a root filesystem based on the image provided", func() {
			bundle := integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id")

			Expect(path.Join(bundle.RootFSPath(), "hello")).To(BeARegularFile())
		})

		Describe("OCI image caching", func() {
			It("caches the image in the store", func() {
				integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id")

				blobPath := path.Join(
					StorePath, "cache", "blobs",
					"sha256-6c1f4533b125f8f825188c4f4ff633a338cfce0db2813124d3d518028baf7d7a",
				)
				Expect(blobPath).To(BeARegularFile())
			})

			It("uses the cached image from the store", func() {
				integration.CreateBundleWSpec(GrootFSBin, StorePath, groot.CreateSpec{
					ID:    "random-id",
					Image: imageURL,
					UIDMappings: []groot.IDMappingSpec{
						groot.IDMappingSpec{NamespaceID: 0, HostID: os.Getuid(), Size: 1},
						groot.IDMappingSpec{NamespaceID: 1, HostID: 100000, Size: 65000},
					},
					GIDMappings: []groot.IDMappingSpec{
						groot.IDMappingSpec{NamespaceID: 0, HostID: os.Getgid(), Size: 1},
						groot.IDMappingSpec{NamespaceID: 1, HostID: 100000, Size: 65000},
					},
				})

				// change the cache
				blobPath := path.Join(
					StorePath, "cache", "blobs",
					"sha256-6c1f4533b125f8f825188c4f4ff633a338cfce0db2813124d3d518028baf7d7a",
				)

				blob, err := os.OpenFile(blobPath, os.O_WRONLY, 0666)
				Expect(err).NotTo(HaveOccurred())
				tarWriter := tar.NewWriter(blob)
				Expect(tarWriter.WriteHeader(&tar.Header{
					Name: "i-hacked-your-cache",
					Mode: 0666,
					Size: int64(len([]byte("cache-hit!"))),
				})).To(Succeed())
				_, err = tarWriter.Write([]byte("cache-hit!"))
				Expect(err).NotTo(HaveOccurred())
				Expect(tarWriter.Close()).To(Succeed())

				bundle := integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id-2")
				Expect(path.Join(bundle.RootFSPath(), "i-hacked-your-cache")).To(BeARegularFile())
			})

			Describe("Unpacked layer caching", func() {
				It("caches the unpacked image in a subvolume with snapshots", func() {
					integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id")

					layerSnapshotPath := filepath.Join(StorePath, "volumes", "sha256:3355e23c079e9b35e4b48075147a7e7e1850b99e089af9a63eed3de235af98ca")
					Expect(ioutil.WriteFile(layerSnapshotPath+"/injected-file", []byte{}, 0666)).To(Succeed())

					bundle := integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id-2")
					Expect(path.Join(bundle.RootFSPath(), "hello")).To(BeARegularFile())
					Expect(path.Join(bundle.RootFSPath(), "injected-file")).To(BeARegularFile())
				})
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

			imageURL = fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.0", proxy.Addr())
		})

		AfterEach(func() {
			proxy.Close()
		})

		It("should create a root filesystem based on the image provided by the private registry", func() {
			bundle := integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id")
			Expect(path.Join(bundle.RootFSPath(), "hello")).To(BeARegularFile())
			Expect(proxy.ReceivedRequests()).NotTo(BeEmpty())
		})
	})
})
