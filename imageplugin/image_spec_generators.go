package imageplugin

import (
	"fmt"

	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	ImageSpecSchemaVersion              = 2
	ImageSpecBaseDirectoryAnnotationKey = "org.cloudfoundry.experimental.image.base-directory"
)

func GenerateImageConfig(layerSHAs ...string) imagespec.Image {
	var digests []digest.Digest
	for _, sha := range layerSHAs {
		digest := toDigest(sha)
		digests = append(digests, digest)
	}
	return imagespec.Image{
		Architecture: "amd64",
		OS:           "linux",
		RootFS: imagespec.RootFS{
			DiffIDs: digests,
			Type:    "layers",
		},
	}
}

func GenerateIndex(manifestSHA string) imagespec.Index {
	return imagespec.Index{
		Versioned: specs.Versioned{SchemaVersion: ImageSpecSchemaVersion},
		Manifests: []imagespec.Descriptor{{
			Digest:    toDigest(manifestSHA),
			MediaType: "application/vnd.oci.image.manifest.v1+json",
		}},
	}
}

func GenerateManifest(layers []Layer, configSHA string) imagespec.Manifest {
	var ociLayers []imagespec.Descriptor
	for _, layer := range layers {
		ociLayer := imagespec.Descriptor{
			Digest:    toDigest(layer.SHA256),
			MediaType: layer.MediaType,
		}

		if layer.URL != "" {
			ociLayer.URLs = []string{layer.URL}
		}

		if layer.BaseDir != "" {
			ociLayer.Annotations = map[string]string{
				ImageSpecBaseDirectoryAnnotationKey: layer.BaseDir,
			}
		}
		ociLayers = append(ociLayers, ociLayer)
	}

	return imagespec.Manifest{
		Versioned: specs.Versioned{SchemaVersion: ImageSpecSchemaVersion},
		Config:    imagespec.Descriptor{MediaType: "application/vnd.oci.image.config.v1+json", Digest: toDigest(configSHA)},
		Layers:    ociLayers,
	}
}

func toDigest(sha256Hex string) digest.Digest {
	return digest.Digest(fmt.Sprintf("sha256:%s", sha256Hex))
}
