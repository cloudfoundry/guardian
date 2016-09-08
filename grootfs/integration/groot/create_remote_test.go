package groot_test

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Create with remote images", func() {
	var imageURL string

	Context("when using the default registry", func() {
		BeforeEach(func() {
			imageURL = "docker:///cfgarden/empty:v0.1.0"
		})

		It("creates a root filesystem based on the image provided", func() {
			bundle := integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id", 0)

			Expect(path.Join(bundle.RootFSPath(), "hello")).To(BeARegularFile())
		})

		It("saves the image.json to the bundle folder", func() {
			bundle := integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id", 0)

			imageJsonPath := path.Join(bundle.Path(), "image.json")
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

		Describe("OCI image caching", func() {
			It("caches the image in the store", func() {
				integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id", 0)

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

				bundle := integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id-2", 0)
				Expect(path.Join(bundle.RootFSPath(), "i-hacked-your-cache")).To(BeARegularFile())
			})

			Describe("Unpacked layer caching", func() {
				It("caches the unpacked image in a subvolume with snapshots", func() {
					integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id", 0)

					layerSnapshotPath := filepath.Join(StorePath, "volumes", "sha256:3355e23c079e9b35e4b48075147a7e7e1850b99e089af9a63eed3de235af98ca")
					Expect(ioutil.WriteFile(layerSnapshotPath+"/injected-file", []byte{}, 0666)).To(Succeed())

					bundle := integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id-2", 0)
					Expect(path.Join(bundle.RootFSPath(), "hello")).To(BeARegularFile())
					Expect(path.Join(bundle.RootFSPath(), "injected-file")).To(BeARegularFile())
				})
			})
		})

		Context("when the image has a version 1 manifest schema", func() {
			BeforeEach(func() {
				imageURL = "docker:///nginx:1.9"
			})

			It("creates a root filesystem based on the image provided", func() {
				bundle := integration.CreateBundle(GrootFSBin, StorePath, imageURL, "random-id", 0)
				Expect(path.Join(bundle.RootFSPath(), "etc/nginx")).To(BeADirectory())
			})
		})
	})

	Context("when downloading the layer fails", func() {
		var (
			proxy    *ghttp.Server
			layerID  string
			requests int
		)

		BeforeEach(func() {
			layerID = "6c1f4533b125f8f825188c4f4ff633a338cfce0db2813124d3d518028baf7d7a"
			dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
			Expect(err).NotTo(HaveOccurred())

			revProxy := httputil.NewSingleHostReverseProxy(dockerHubUrl)

			oldDirector := revProxy.Director
			revProxy.Director = func(req *http.Request) {
				oldDirector(req)
				req.Host = "registry-1.docker.io"

				// It matches only the first layer (ignoring manifest and config)
				re, _ := regexp.Compile(fmt.Sprintf(`\/v2\/cfgarden\/empty\/blobs\/sha256:%s`, layerID))
				match := re.FindStringSubmatch(req.URL.Path)
				if match != nil {
					req.Header.Add("X-TEST-GROOTFS", "true")
				}
			}

			revProxy.Transport = &integration.CustomRoundTripper{
				RoundTripFn: func(req *http.Request) (*http.Response, error) {
					if req.Header.Get("X-TEST-GROOTFS") != "true" || requests == 1 {
						return http.DefaultTransport.RoundTrip(req)
					}

					var body io.ReadCloser = ioutil.NopCloser(bytes.NewBufferString("i-am-groot"))
					response := &http.Response{
						StatusCode:    http.StatusOK,
						Proto:         req.Proto,
						ProtoMajor:    req.ProtoMajor,
						ProtoMinor:    req.ProtoMinor,
						Header:        make(map[string][]string),
						ContentLength: int64(1024),
						Body:          body,
					}
					requests++
					return response, nil
				},
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

		It("does not leak corrupted state", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", "--insecure-registry", proxy.Addr(), imageURL, "random-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 12*time.Second).Should(gexec.Exit(1))
			Expect(path.Join(StorePath, "cache", "blobs", fmt.Sprintf("sha256-%s", layerID))).NotTo(BeARegularFile())

			// Can still be used succesfully later
			cmd = exec.Command(GrootFSBin, "--store", StorePath, "create", "--insecure-registry", proxy.Addr(), imageURL, "random-id")
			sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 12*time.Second).Should(gexec.Exit(0))
			Expect(path.Join(StorePath, "cache", "blobs", fmt.Sprintf("sha256-%s", layerID))).To(BeARegularFile())
		})
	})

	Context("when a private registry is used", func() {
		var (
			proxy          *ghttp.Server
			requestedBlobs map[string]bool
		)

		BeforeEach(func() {
			dockerHubUrl, err := url.Parse("https://registry-1.docker.io")
			Expect(err).NotTo(HaveOccurred())

			revProxy := httputil.NewSingleHostReverseProxy(dockerHubUrl)

			requestedBlobs = make(map[string]bool)

			// Dockerhub returns 503 if the host is set to localhost
			// as it happens with the reverse proxy
			oldDirector := revProxy.Director
			revProxy.Director = func(req *http.Request) {
				oldDirector(req)
				req.Host = "registry-1.docker.io"

				// log blob
				re, _ := regexp.Compile(`\/v2\/cfgarden\/empty\/blobs\/sha256:([a-f0-9]*)`)
				match := re.FindStringSubmatch(req.URL.Path)
				if match != nil {
					requestedBlobs[match[1]] = true
				}
			}

			proxy = ghttp.NewTLSServer()
			ourRegexp, err := regexp.Compile(`.*`)
			Expect(err).NotTo(HaveOccurred())
			proxy.RouteToHandler("GET", ourRegexp, revProxy.ServeHTTP)

			imageURL = fmt.Sprintf("docker://%s/cfgarden/empty:v0.1.1", proxy.Addr())
		})

		AfterEach(func() {
			proxy.Close()
		})

		It("fails to fetch the image", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", imageURL, "random-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 12*time.Second).Should(gexec.Exit(1))
			Eventually(sess).Should(gbytes.Say("TLS validation of insecure registry failed"))
		})

		Context("when it's provided as a valid insecure registry", func() {
			It("should create a root filesystem based on the image provided by the private registry", func() {
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", "--insecure-registry", proxy.Addr(), imageURL, "random-id")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess, 12*time.Second).Should(gexec.Exit(0))

				rootFSPath := strings.TrimSpace(string(sess.Out.Contents())) + "/rootfs"
				Expect(path.Join(rootFSPath, "hello")).To(BeARegularFile())
				Expect(proxy.ReceivedRequests()).NotTo(BeEmpty())

				Expect(requestedBlobs).To(HaveLen(3))
			})
		})
	})
})
