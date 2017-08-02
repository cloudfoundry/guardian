package source_test

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"

	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"
	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher/source"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	digestpkg "github.com/opencontainers/go-digest"
)

var _ = Describe("Layer source: OCI", func() {
	var (
		trustedRegistries []string
		layerSource       *source.LayerSource

		logger       *lagertest.TestLogger
		baseImageURL *url.URL

		configBlob           string
		expectedLayersDigest []layer_fetcher.Layer
		expectedDiffIds      []digestpkg.Digest
		manifest             layer_fetcher.Manifest
		workDir              string
	)

	BeforeEach(func() {
		trustedRegistries = []string{}

		configBlob = "sha256:10c8f0eb9d1af08fe6e3b8dbd29e5aa2b6ecfa491ecd04ed90de19a4ac22de7b"
		expectedLayersDigest = []layer_fetcher.Layer{
			layer_fetcher.Layer{
				BlobID: "sha256:56bec22e355981d8ba0878c6c2f23b21f422f30ab0aba188b54f1ffeff59c190",
				Size:   668151,
			},
			layer_fetcher.Layer{
				BlobID: "sha256:ed2d7b0f6d7786230b71fd60de08a553680a9a96ab216183bcc49c71f06033ab",
				Size:   124,
			},
		}
		expectedDiffIds = []digestpkg.Digest{
			digestpkg.NewDigestFromHex("sha256", "e88b3f82283bc59d5e0df427c824e9f95557e661fcb0ea15fb0fb6f97760f9d9"),
			digestpkg.NewDigestFromHex("sha256", "1e664bbd066a13dc6e8d9503fe0d439e89617eaac0558a04240bcbf4bd969ff9"),
		}

		manifest = layer_fetcher.Manifest{
			ConfigCacheKey: configBlob,
			SchemaVersion:  2,
		}

		logger = lagertest.NewTestLogger("test-layer-source")
		var err error
		workDir, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		baseImageURL, err = url.Parse(fmt.Sprintf("oci:///%s/../../../integration/assets/oci-test-image/opq-whiteouts-busybox:latest", workDir))
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		layerSource = source.NewLayerSource("", "", trustedRegistries)
	})

	Describe("Manifest", func() {
		It("fetches the manifest", func() {
			manifest, err := layerSource.Manifest(logger, baseImageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.ConfigCacheKey).To(Equal(configBlob))

			Expect(manifest.Layers).To(HaveLen(2))
			Expect(manifest.Layers[0]).To(Equal(expectedLayersDigest[0]))
			Expect(manifest.Layers[1]).To(Equal(expectedLayersDigest[1]))
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				baseImageURL, err := url.Parse("oci://///cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = layerSource.Manifest(logger, baseImageURL)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the image does not exist", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("oci:///cfgarden/non-existing-image")
				Expect(err).NotTo(HaveOccurred())
			})

			It("wraps the containers/image with a useful error", func() {
				_, err := layerSource.Manifest(logger, baseImageURL)
				Expect(err.Error()).To(MatchRegexp("^fetching image reference"))
			})

			It("logs the original error message", func() {
				_, err := layerSource.Manifest(logger, baseImageURL)
				Expect(err).To(HaveOccurred())

				Expect(logger).To(gbytes.Say("fetching-image-reference-failed"))
				Expect(logger).To(gbytes.Say("parsing url failed: lstat /cfgarden: no such file or directory"))
			})
		})
	})

	Describe("Config", func() {
		It("fetches the config", func() {
			config, err := layerSource.Config(logger, baseImageURL, manifest)
			Expect(err).NotTo(HaveOccurred())

			Expect(config.RootFS.DiffIDs).To(HaveLen(2))
			Expect(config.RootFS.DiffIDs[0]).To(Equal(expectedDiffIds[0]))
			Expect(config.RootFS.DiffIDs[1]).To(Equal(expectedDiffIds[1]))
		})

		Context("when the image or the config blob does not exist", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse(fmt.Sprintf("oci:///%s/../../../integration/assets/oci-test-image/invalid-config:latest", workDir))
				Expect(err).NotTo(HaveOccurred())
			})

			It("retuns an error", func() {
				_, err := layerSource.Config(logger, baseImageURL, manifest)
				Expect(err).To(MatchError(ContainSubstring("fetching config blob")))
			})
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				baseImageURL, err := url.Parse("oci://////cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = layerSource.Config(logger, baseImageURL, manifest)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})
	})

	Describe("Blob", func() {
		It("downloads a blob", func() {
			blobPath, size, err := layerSource.Blob(logger, baseImageURL, expectedLayersDigest[0].BlobID)
			Expect(err).NotTo(HaveOccurred())

			blobReader, err := os.Open(blobPath)
			Expect(err).NotTo(HaveOccurred())

			buffer := gbytes.NewBuffer()
			cmd := exec.Command("tar", "tzv")
			cmd.Stdin = blobReader
			sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(int64(668151)))

			Eventually(buffer).Should(gbytes.Say("etc/localtime"))
			Eventually(sess).Should(gexec.Exit(0))
		})

		Context("when the blob has an invalid checksum", func() {
			It("returns an error", func() {
				_, _, err := layerSource.Blob(logger, baseImageURL, "sha256:steamed-blob")
				Expect(err).To(MatchError(ContainSubstring("invalid checksum digest format")))
			})
		})

		Context("when the blob is corrupted", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse(fmt.Sprintf("oci:///%s/../../../integration/assets/oci-test-image/corrupted:latest", workDir))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				_, _, err := layerSource.Blob(logger, baseImageURL, expectedLayersDigest[0].BlobID)
				Expect(err).To(MatchError(ContainSubstring("invalid checksum: layer is corrupted")))
			})
		})
	})
})
