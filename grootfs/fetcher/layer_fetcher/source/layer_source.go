package source // import "code.cloudfoundry.org/grootfs/fetcher/layer_fetcher/source"

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"
	"code.cloudfoundry.org/lager"
	"github.com/Sirupsen/logrus"
	_ "github.com/containers/image/docker"
	manifestpkg "github.com/containers/image/manifest"
	_ "github.com/containers/image/oci/layout"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	digestpkg "github.com/opencontainers/go-digest"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	errorspkg "github.com/pkg/errors"
)

const MAX_BLOB_RETRIES = 3

type LayerSource struct {
	trustedRegistries []string
	username          string
	password          string
}

func NewLayerSource(username, password string, trustedRegistries []string) *LayerSource {
	return &LayerSource{
		username:          username,
		password:          password,
		trustedRegistries: trustedRegistries,
	}
}

func (s *LayerSource) Manifest(logger lager.Logger, baseImageURL *url.URL) (layer_fetcher.Manifest, error) {
	logger = logger.Session("fetching-image-manifest", lager.Data{"baseImageURL": baseImageURL})
	logger.Info("starting")
	defer logger.Info("ending")

	img, err := s.image(logger, baseImageURL)
	if err != nil {
		logger.Error("fetching-image-reference-failed", err)

		return layer_fetcher.Manifest{}, errorspkg.Wrap(err, "fetching image reference")
	}

	contents, mimeType, err := img.Manifest()
	if err != nil {
		logger.Error("fetching-manifest-failed", err)
		return layer_fetcher.Manifest{}, errorspkg.Wrap(err, "fetching manifest")
	}

	var manifest layer_fetcher.Manifest
	switch mimeType {
	case manifestpkg.DockerV2Schema1MediaType, manifestpkg.DockerV2Schema1SignedMediaType:
		logger.Debug("docker-image-version-2-schema-1")
		manifest, err = s.parseSchemaV1Manifest(logger, contents)

	case specsv1.MediaTypeImageManifest, manifestpkg.DockerV2Schema2MediaType:
		logger.Debug("docker-image-version-2-schema-2")
		manifest, err = s.parseSchemaV2Manifest(logger, contents)

	default:
		return layer_fetcher.Manifest{}, errorspkg.New(fmt.Sprintf("unknown media type '%s'", mimeType))
	}

	return manifest, err
}

func (s *LayerSource) Config(logger lager.Logger, baseImageURL *url.URL, manifest layer_fetcher.Manifest) (specsv1.Image, error) {
	logger = logger.Session("fetching-image-config", lager.Data{
		"baseImageURL": baseImageURL,
		"configDigest": manifest.ConfigCacheKey,
	})
	logger.Info("starting")
	defer logger.Info("ending")

	var (
		config specsv1.Image
		err    error
	)
	switch manifest.SchemaVersion {
	case 1:
		logger.Debug("docker-image-version-2-schema-1")
		config, err = s.parseSchemaV1Config(logger, manifest)
		if err != nil {
			return specsv1.Image{}, err
		}
	case 2:
		logger.Debug("docker-image-version-2-schema-2")
		config, err = s.parseSchemaV2Config(logger, baseImageURL, manifest.ConfigCacheKey)
		if err != nil {
			return specsv1.Image{}, err
		}
	default:
		return specsv1.Image{}, errorspkg.Errorf("schema version not supported (%d)", manifest.SchemaVersion)
	}

	return config, nil
}

func (s *LayerSource) Blob(logger lager.Logger, baseImageURL *url.URL, digest string) (string, int64, error) {
	logrus.SetOutput(os.Stderr)
	logger = logger.Session("streaming-blob", lager.Data{
		"baseImageURL": baseImageURL,
		"digest":       digest,
	})
	logger.Info("starting")
	defer logger.Info("ending")

	imgSrc, err := s.imageSource(logger, baseImageURL)
	if err != nil {
		return "", 0, err
	}

	blobInfo := types.BlobInfo{Digest: digestpkg.Digest(digest)}

	blob, size, err := s.getBlobWithRetries(imgSrc, blobInfo, logger)
	if err != nil {
		return "", 0, err
	}
	logger.Debug("got-blob-stream", lager.Data{"digest": digest, "size": size})

	blobTempFile, err := ioutil.TempFile("", fmt.Sprintf("blob-%s", digest))
	if err != nil {
		return "", 0, err
	}
	defer blobTempFile.Close()

	blobReader := io.TeeReader(blob, blobTempFile)
	if !s.checkCheckSum(logger, blobReader, digest) {
		return "", 0, errorspkg.Errorf("invalid checksum: layer is corrupted `%s`", digest)
	}
	return blobTempFile.Name(), size, nil
}

func (s *LayerSource) getBlobWithRetries(imgSrc types.ImageSource, blobInfo types.BlobInfo, logger lager.Logger) (io.ReadCloser, int64, error) {
	var err error
	for i := 0; i < MAX_BLOB_RETRIES; i++ {
		logger.Info("attempt-get-blob", lager.Data{"attempt": i + 1})
		blob, size, e := imgSrc.GetBlob(blobInfo)
		if e == nil {
			return blob, size, nil
		}
		err = e
	}

	return nil, 0, err
}

func (s *LayerSource) checkCheckSum(logger lager.Logger, data io.Reader, digest string) bool {
	var (
		actualSize int64
		err        error
	)

	hash := sha256.New()
	if actualSize, err = io.Copy(hash, data); err != nil {
		logger.Error("failed-to-hash-data", err)
		return false
	}
	logger.Debug("got-blob", lager.Data{"actualSize": actualSize})

	digestID := strings.Split(digest, ":")[1]
	blobContentsSha := hex.EncodeToString(hash.Sum(nil))
	logger.Debug("checking-checksum", lager.Data{
		"digestIDChecksum":   digestID,
		"downloadedChecksum": blobContentsSha,
	})

	return digestID == blobContentsSha
}

func (s *LayerSource) skipTLSValidation(baseImageURL *url.URL) bool {
	for _, trustedRegistry := range s.trustedRegistries {
		if baseImageURL.Host == trustedRegistry {
			return true
		}
	}

	return false
}

func (s *LayerSource) parseSchemaV1Manifest(logger lager.Logger, rawManifest []byte) (layer_fetcher.Manifest, error) {
	var dockerManifest layer_fetcher.SchemaV1Manifest
	if err := json.Unmarshal(rawManifest, &dockerManifest); err != nil {
		logger.Error("parsing-manifest-failed", err, lager.Data{"manifest": string(rawManifest)})
		return layer_fetcher.Manifest{}, errorspkg.Wrap(err, "parsing manifest")
	}

	manifest := layer_fetcher.Manifest{}
	for _, layer := range dockerManifest.FSLayers {
		manifest.Layers = append([]layer_fetcher.Layer{layer_fetcher.Layer{BlobID: layer["blobSum"]}}, manifest.Layers...)
	}

	for _, history := range dockerManifest.History {
		manifest.V1Compatibility = append([]string{history.V1Compatibility}, manifest.V1Compatibility...)
	}

	v1Config := manifest.V1Compatibility[len(manifest.V1Compatibility)-1]
	configSha := sha256.Sum256([]byte(v1Config))
	manifest.ConfigCacheKey = fmt.Sprintf("sha256:%s", hex.EncodeToString(configSha[:32]))
	manifest.SchemaVersion = 1

	return manifest, nil
}

func (s *LayerSource) parseSchemaV2Manifest(logger lager.Logger, rawManifest []byte) (layer_fetcher.Manifest, error) {
	var ociManifest specsv1.Manifest
	if err := json.Unmarshal(rawManifest, &ociManifest); err != nil {
		logger.Error("parsing-manifest-failed", err, lager.Data{"manifest": string(rawManifest)})
		return layer_fetcher.Manifest{}, errorspkg.Wrap(err, "parsing manifest")
	}

	manifest := layer_fetcher.Manifest{
		ConfigCacheKey: ociManifest.Config.Digest.String(),
	}
	for _, layer := range ociManifest.Layers {
		manifest.Layers = append(manifest.Layers, layer_fetcher.Layer{BlobID: layer.Digest.String(), Size: layer.Size})
	}

	manifest.SchemaVersion = 2
	return manifest, nil
}

func (s *LayerSource) parseSchemaV2Config(logger lager.Logger, baseImageURL *url.URL, configDigest string) (specsv1.Image, error) {
	imgSrc, err := s.imageSource(logger, baseImageURL)
	if err != nil {
		return specsv1.Image{}, err
	}

	d := digestpkg.Digest(configDigest)
	stream, _, err := imgSrc.GetBlob(types.BlobInfo{Digest: d})
	if err != nil {
		logger.Error("fetching-config-failed", err)
		return specsv1.Image{}, errorspkg.Wrap(err, "fetching config blob")
	}

	var config specsv1.Image
	if err := json.NewDecoder(stream).Decode(&config); err != nil {
		logger.Error("parsing-config-failed", err)
		return specsv1.Image{}, errorspkg.Wrap(err, "parsing image config")
	}

	return config, nil
}

func (s *LayerSource) parseSchemaV1Config(logger lager.Logger, manifest layer_fetcher.Manifest) (specsv1.Image, error) {
	if len(manifest.V1Compatibility) == 0 {
		logger.Error("v1-manifest-validation-failed", errorspkg.New("v1compatibility has no layers"), lager.Data{"manifest": manifest})
		return specsv1.Image{}, errorspkg.New("V1Compatibility is empty for the manifest")
	}

	var config specsv1.Image
	v1Config := manifest.V1Compatibility[len(manifest.V1Compatibility)-1]
	if err := json.Unmarshal([]byte(v1Config), &config); err != nil {
		logger.Error("parsing-manifest-v1-compatibility-failed", err)
		return specsv1.Image{}, errorspkg.Wrap(err, "parsing manifest V1Compatibility")
	}

	for _, rawHistory := range manifest.V1Compatibility {
		var v1Compatibility layer_fetcher.V1Compatibility
		if err := json.Unmarshal([]byte(rawHistory), &v1Compatibility); err != nil {
			logger.Error("parsing-manifest-v1-compatibility-failed", err)
			return specsv1.Image{}, errorspkg.Wrap(err, "parsing manifest V1Compatibility")
		}

		digest := digestpkg.NewDigestFromHex("sha256", v1Compatibility.ID)
		config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, digest)
	}

	return config, nil
}

func (s *LayerSource) reference(logger lager.Logger, baseImageURL *url.URL) (types.ImageReference, error) {
	refString := "/"
	if baseImageURL.Host != "" {
		refString += "/" + baseImageURL.Host
	}
	refString += baseImageURL.Path

	logger.Debug("parsing-reference", lager.Data{"refString": refString})
	transport := transports.Get(baseImageURL.Scheme)
	ref, err := transport.ParseReference(refString)
	if err != nil {
		return nil, errorspkg.Wrap(err, "parsing url failed")
	}

	return ref, nil
}

func (s *LayerSource) image(logger lager.Logger, baseImageURL *url.URL) (types.Image, error) {
	ref, err := s.reference(logger, baseImageURL)
	if err != nil {
		return nil, err
	}

	skipTLSValidation := s.skipTLSValidation(baseImageURL)
	logger.Debug("new-image", lager.Data{"skipTLSValidation": skipTLSValidation})
	img, err := ref.NewImage(&types.SystemContext{
		DockerInsecureSkipTLSVerify: skipTLSValidation,
		DockerAuthConfig: &types.DockerAuthConfig{
			Username: s.username,
			Password: s.password,
		},
	})
	if err != nil {
		return nil, errorspkg.Wrap(err, "creating reference")
	}

	return img, nil
}

func (s *LayerSource) imageSource(logger lager.Logger, baseImageURL *url.URL) (types.ImageSource, error) {
	ref, err := s.reference(logger, baseImageURL)
	if err != nil {
		return nil, err
	}

	skipTLSValidation := s.skipTLSValidation(baseImageURL)

	imgSrc, err := ref.NewImageSource(&types.SystemContext{
		DockerInsecureSkipTLSVerify: skipTLSValidation,
		DockerAuthConfig: &types.DockerAuthConfig{
			Username: s.username,
			Password: s.password,
		},
	}, preferedMediaTypes())
	if err != nil {
		return nil, errorspkg.Wrap(err, "creating reference")
	}
	logger.Debug("new-image", lager.Data{"skipTLSValidation": skipTLSValidation})

	return imgSrc, nil
}

func preferedMediaTypes() []string {
	return []string{
		specsv1.MediaTypeImageManifest,
		manifestpkg.DockerV2Schema2MediaType,
	}
}
