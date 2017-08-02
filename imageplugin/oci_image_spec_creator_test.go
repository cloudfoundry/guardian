package imageplugin_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/guardian/imageplugin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/image-spec/specs-go"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("OciImageSpecCreator", func() {
	var (
		tmpDir string

		depotDir           string
		configGenerator    func(layerSHAs ...string) imagespec.Image
		createdConfig      = imagespec.Image{Author: "some-idiosyncratic-author-string"}
		createdConfigSHA   string
		manifestGenerator  func(layers []imageplugin.Layer, configSHA string) imagespec.Manifest
		createdManifest    = imagespec.Manifest{Versioned: specs.Versioned{SchemaVersion: 165}}
		createdManifestSHA string
		indexGenerator     func(manifestSHA string) imagespec.Index
		createdIndex       = imagespec.Index{Versioned: specs.Versioned{SchemaVersion: 42}}

		creator *imageplugin.OCIImageSpecCreator

		rootFSURLStr   string
		rootFSBaseDir  string
		bottomLayerSHA string
		handle         = "foobarbazbarry"

		newURL    *url.URL
		createErr error
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "imageplugin-tests")
		Expect(err).NotTo(HaveOccurred())

		rootFSBaseDir = filepath.Join(tmpDir, "rootfs-base")
		rootFSURLStr = fmt.Sprintf(
			"preloaded+layer://%s?layer=https://layers.com/layer.tgz&layer_path=/untar/here&layer_digest=some-digest",
			forwardSlashesOnly(rootFSBaseDir),
		)
		rootFSBasePath := normalisePath(rootFSBaseDir)
		Expect(os.MkdirAll(rootFSBaseDir, 0700)).To(Succeed())
		rootFSBasePathFile, err := os.Stat(rootFSBaseDir)
		Expect(err).NotTo(HaveOccurred())
		rootFSBasePathMtime := rootFSBasePathFile.ModTime().UnixNano()
		rootFSBasePathSHABytes := sha256.Sum256([]byte(rootFSBasePath))
		rootFSBasePathSHA := hex.EncodeToString(rootFSBasePathSHABytes[:])
		bottomLayerSHA = fmt.Sprintf("%s-%d", rootFSBasePathSHA, rootFSBasePathMtime)

		depotDir = filepath.Join(tmpDir, "depot")

		createdConfigSHA = shaOf(createdConfig)
		configGenerator = func(layerSHAs ...string) imagespec.Image {
			Expect(layerSHAs).To(Equal([]string{
				bottomLayerSHA,
				"some-digest",
			}))
			return createdConfig
		}

		createdManifestSHA = shaOf(createdManifest)
		manifestGenerator = func(layers []imageplugin.Layer, configSHA string) imagespec.Manifest {
			Expect(layers).To(Equal([]imageplugin.Layer{
				{
					URL:    "file://" + rootFSBasePath,
					SHA256: bottomLayerSHA,
				},
				{
					URL:     "https://layers.com/layer.tgz",
					SHA256:  "some-digest",
					BaseDir: "/untar/here",
				},
			}))
			Expect(configSHA).To(Equal(createdConfigSHA))
			return createdManifest
		}

		indexGenerator = func(manifestSHA string) imagespec.Index {
			Expect(manifestSHA).To(Equal(createdManifestSHA))
			return createdIndex
		}

		creator = &imageplugin.OCIImageSpecCreator{
			DepotDir:             depotDir,
			ImageConfigGenerator: configGenerator,
			ManifestGenerator:    manifestGenerator,
			IndexGenerator:       indexGenerator,
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		rootFSURL, err := url.Parse(rootFSURLStr)
		Expect(err).NotTo(HaveOccurred())
		newURL, createErr = creator.CreateImageSpec(rootFSURL, handle)
	})

	It("returns no error", func() {
		Expect(createErr).NotTo(HaveOccurred())
	})

	It("creates the OCI image directory", func() {
		ociImagePath := filepath.Join(depotDir, handle, "image")
		Expect(ociImagePath).To(BeADirectory())

		blobsPath := filepath.Join(ociImagePath, "blobs", "sha256")

		configPath := filepath.Join(blobsPath, createdConfigSHA)
		var imageConfig imagespec.Image
		unmarshalJSONFromFile(configPath, &imageConfig)
		Expect(imageConfig).To(Equal(createdConfig))

		manifestPath := filepath.Join(blobsPath, createdManifestSHA)
		var manifest imagespec.Manifest
		unmarshalJSONFromFile(manifestPath, &manifest)
		Expect(manifest).To(Equal(createdManifest))

		indexPath := filepath.Join(ociImagePath, "index.json")
		var index imagespec.Index
		unmarshalJSONFromFile(indexPath, &index)
		Expect(index).To(Equal(createdIndex))
	})

	It("returns the OCI image URL", func() {
		Expect(newURL.String()).To(Equal(fmt.Sprintf("oci://%s/%s/image", forwardSlashesOnly(depotDir), handle)))
	})

	Context("when the URL scheme is not preloaded+layer", func() {
		BeforeEach(func() {
			rootFSURLStr = "https://wrong.io"
		})

		It("returns an error", func() {
			Expect(createErr).To(MatchError("scheme 'https' not supported: expected preloaded+layer"))
		})
	})

	Context("when the query param 'layer' is not set", func() {
		BeforeEach(func() {
			rootFSURLStr = "preloaded+layer:///rootfs/path?layer_path=/untar/here&layer_digest=some-digest"
		})

		It("returns an error", func() {
			Expect(createErr).To(MatchError(ContainSubstring("no query parameter 'layer'")))
		})
	})

	Context("when the query param 'layer_path' is not set", func() {
		BeforeEach(func() {
			rootFSURLStr = "preloaded+layer:///rootfs/path?layer=some_layer&layer_digest=some-digest"
		})

		It("returns an error", func() {
			Expect(createErr).To(MatchError(ContainSubstring("no query parameter 'layer_path'")))
		})
	})

	Context("when the query param 'layer_digest' is not set", func() {
		BeforeEach(func() {
			rootFSURLStr = "preloaded+layer:///rootfs/path?layer=some_layer&layer_path=/untar/here"
		})

		It("returns an error", func() {
			Expect(createErr).To(MatchError(ContainSubstring("no query parameter 'layer_digest'")))
		})
	})
})

func shaOf(obj interface{}) string {
	serialisedObj, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	sha := sha256.Sum256(serialisedObj)
	return hex.EncodeToString(sha[:])
}

func unmarshalJSONFromFile(path string, into interface{}) {
	contents, err := ioutil.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	Expect(json.Unmarshal(contents, into)).To(Succeed())
}

// On Windows, temp dir paths will contain backslashes.
// However, a valid Windows file URI uses forward slashes, e.g.
// file://C:/some/path
func forwardSlashesOnly(pathname string) string {
	return strings.Replace(pathname, `\`, "/", -1)
}

// In a file:// *url.URL on Windows, the path is only the part after the drive
// letter (which is the host). E.g. for a URL file://C:/some/path, the path
// component is /some/path. The host is "C:".
func normalisePath(pathname string) string {
	return forwardSlashesOnly(strings.TrimLeft(pathname, "C:"))
}
