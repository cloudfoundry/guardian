package imageplugin_test

import (
	"code.cloudfoundry.org/guardian/imageplugin"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("GenerateImageConfig", func() {
	It("creates an image config", func() {
		config := imageplugin.GenerateImageConfig("sha1", "sha2")
		Expect(config).To(Equal(imagespec.Image{
			OS:           "linux",
			Architecture: "amd64",
			RootFS: imagespec.RootFS{
				DiffIDs: []digest.Digest{"sha256:sha1", "sha256:sha2"},
				Type:    "layers",
			},
		}))
	})
})

var _ = Describe("GenerateManifest", func() {
	It("creates a manifest", func() {
		layerWithoutBaseDir := imageplugin.Layer{
			URL:    "url1",
			SHA256: "sha1",
		}

		layerWithBaseDir := imageplugin.Layer{
			URL:     "url2",
			SHA256:  "sha2",
			BaseDir: "/base/dir",
		}

		manifest := imageplugin.GenerateManifest(
			[]imageplugin.Layer{layerWithoutBaseDir, layerWithBaseDir},
			"some-config-sha",
		)
		Expect(manifest).To(Equal(imagespec.Manifest{
			Versioned: specs.Versioned{SchemaVersion: imageplugin.ImageSpecSchemaVersion},
			Config: imagespec.Descriptor{
				Digest: "sha256:some-config-sha",
			},
			Layers: []imagespec.Descriptor{
				{
					Digest: "sha256:sha1",
					URLs:   []string{"url1"},
				},
				{
					Digest: "sha256:sha2",
					URLs:   []string{"url2"},
					Annotations: map[string]string{
						imageplugin.ImageSpecBaseDirectoryAnnotationKey: "/base/dir",
					},
				},
			},
		}))
	})
})

var _ = Describe("GenerateIndex", func() {
	It("creates an index", func() {
		index := imageplugin.GenerateIndex("some-manifest-sha")
		Expect(index).To(Equal(imagespec.Index{
			Versioned: specs.Versioned{SchemaVersion: imageplugin.ImageSpecSchemaVersion},
			Manifests: []imagespec.Descriptor{{
				Digest: digest.Digest("sha256:some-manifest-sha"),
			}},
		}))
	})
})
