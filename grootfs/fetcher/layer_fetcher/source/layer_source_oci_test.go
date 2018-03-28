package source_test

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"

	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher/source"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/containers/image/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Layer source: OCI", func() {
	var (
		layerSource source.LayerSource

		logger       *lagertest.TestLogger
		baseImageURL *url.URL

		configBlob    string
		layerInfos    []groot.LayerInfo
		workDir       string
		systemContext types.SystemContext

		skipOCILayerValidation bool
	)

	BeforeEach(func() {
		skipOCILayerValidation = false

		configBlob = "sha256:18c5d86cd64efe05ea5e2e18de4b48848a4f5a425235097f34e17f6aca81f4f3"
		layerInfos = []groot.LayerInfo{
			{
				BlobID:    "sha256:56bec22e355981d8ba0878c6c2f23b21f422f30ab0aba188b54f1ffeff59c190",
				DiffID:    "e88b3f82283bc59d5e0df427c824e9f95557e661fcb0ea15fb0fb6f97760f9d9",
				Size:      668151,
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			},
			{
				BlobID:    "sha256:e8fbc9c5bf16d3409f75a9d0f0751d90ab562565335b793673e906efcc7bd7c8",
				DiffID:    "8c3258a61af653812528b6d303bc126b5ef910cb54b6e20a0b1ed52887a0cef1",
				Size:      172,
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			},
		}

		logger = lagertest.NewTestLogger("test-layer-source")
		var err error
		workDir, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		baseImageURL, err = url.Parse(fmt.Sprintf("oci:///%s/../../../integration/assets/oci-test-image/opq-whiteouts-busybox:latest", workDir))
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		layerSource = source.NewLayerSource(systemContext, skipOCILayerValidation, false, 0, baseImageURL)
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

		It("contains the config", func() {
			manifest, err := layerSource.Manifest(logger)
			Expect(err).NotTo(HaveOccurred())

			config, err := manifest.OCIConfig()
			Expect(err).NotTo(HaveOccurred())

			Expect(config.RootFS.DiffIDs).To(HaveLen(2))
			Expect(config.RootFS.DiffIDs[0].Hex()).To(Equal(layerInfos[0].DiffID))
			Expect(config.RootFS.DiffIDs[1].Hex()).To(Equal(layerInfos[1].DiffID))
		})

		Context("when the image url is invalid", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("oci://///cfgarden/empty:v0.1.0")
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
				baseImageURL, err = url.Parse("oci:///cfgarden/non-existing-image")
				Expect(err).NotTo(HaveOccurred())
			})

			It("wraps the containers/image with a useful error", func() {
				_, err := layerSource.Manifest(logger)
				Expect(err.Error()).To(MatchRegexp("^fetching image reference"))
			})

			It("logs the original error message", func() {
				_, err := layerSource.Manifest(logger)
				Expect(err).To(HaveOccurred())

				Expect(logger).To(gbytes.Say("fetching-image-reference-failed"))
				Expect(logger).To(gbytes.Say("parsing url failed: lstat /cfgarden: no such file or directory"))
			})
		})

		Context("when the config blob does not exist", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse(fmt.Sprintf("oci:///%s/../../../integration/assets/oci-test-image/invalid-config:latest", workDir))
				Expect(err).NotTo(HaveOccurred())
			})

			It("retuns an error", func() {
				_, err := layerSource.Manifest(logger)
				Expect(err).To(MatchError(ContainSubstring("creating image")))
			})
		})
	})

	Describe("Blob", func() {
		It("downloads a blob", func() {
			blobPath, size, err := layerSource.Blob(logger, layerInfos[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(int64(668151)))

			blobReader, err := os.Open(blobPath)
			Expect(err).NotTo(HaveOccurred())

			buffer := gbytes.NewBuffer()
			cmd := exec.Command("tar", "tv")
			cmd.Stdin = blobReader
			sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess, "2s").Should(gexec.Exit(0))
			Expect(string(buffer.Contents())).To(ContainSubstring("etc/localtime"))
		})

		Context("when the blob has an invalid checksum", func() {
			It("returns an error", func() {
				_, _, err := layerSource.Blob(logger, groot.LayerInfo{BlobID: "sha256:steamed-blob"})
				Expect(err).To(MatchError(ContainSubstring("invalid checksum digest format")))
			})
		})

		Context("when the blob is corrupted", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse(fmt.Sprintf("oci:///%s/../../../integration/assets/oci-test-image/corrupted:latest", workDir))
				Expect(err).NotTo(HaveOccurred())
				layerInfos[0].Size = 668551
			})

			It("returns an error", func() {
				_, _, err := layerSource.Blob(logger, layerInfos[0])
				Expect(err).To(MatchError(ContainSubstring("layerID digest mismatch")))
			})
		})

		Context("when skipOCILayerValidation is set to true", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse(fmt.Sprintf("oci:///%s/../../../integration/assets/oci-test-image/corrupted:latest", workDir))
				Expect(err).NotTo(HaveOccurred())
				layerInfos[0].Size = 668551
				skipOCILayerValidation = true
			})

			It("does not validate against checksums and does not return an error", func() {
				_, _, err := layerSource.Blob(logger, layerInfos[0])
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the actual blob size is different than the layersize in the manifest", func() {
				BeforeEach(func() {
					layerInfos[0].Size = 100
				})

				It("does not validate layer size", func() {
					_, _, err := layerSource.Blob(logger, layerInfos[0])
					Expect(err).NotTo(HaveOccurred())
				})
			})

		})

		Context("when the blob doesn't match the diffID", func() {
			BeforeEach(func() {
				layerInfos[0].DiffID = "0000000000000000000000000000000000000000000000000000000000000000"
			})

			It("returns an error", func() {
				_, _, err := layerSource.Blob(logger, layerInfos[0])
				Expect(err).To(MatchError(ContainSubstring("diffID digest mismatch")))
			})
		})

		Context("when the actual blob size is greater than the layersize in the manifest", func() {
			BeforeEach(func() {
				layerInfos[0].Size = 100
			})

			It("returns an error", func() {
				_, _, err := layerSource.Blob(logger, layerInfos[0])
				Expect(err).To(MatchError(ContainSubstring("layer size is greater than the value in the manifest")))
			})
		})

		Context("when the actual blob size is less than the layersize in the manifest", func() {
			BeforeEach(func() {
				layerInfos[0].Size = 10000000000000
			})

			It("returns an error", func() {
				_, _, err := layerSource.Blob(logger, layerInfos[0])
				Expect(err).To(MatchError(ContainSubstring("layer size is less than the value in the manifest")))
			})
		})

		Context("when the uncompressed layer size is bigger that the quota", func() {
			BeforeEach(func() {

			})
		})
	})
})
