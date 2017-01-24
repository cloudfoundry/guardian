package remote // import "code.cloudfoundry.org/grootfs/fetcher/remote"

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/Sirupsen/logrus"
	"github.com/containers/image/docker"
	manifestpkg "github.com/containers/image/manifest"
	"github.com/containers/image/types"
	digestpkg "github.com/docker/distribution/digest"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	errorspkg "github.com/pkg/errors"
)

type DockerSource struct {
	trustedRegistries []string
	username          string
	password          string
}

func NewDockerSource(username, password string, trustedRegistries []string) *DockerSource {
	return &DockerSource{
		username:          username,
		password:          password,
		trustedRegistries: trustedRegistries,
	}
}

func (s *DockerSource) Manifest(logger lager.Logger, baseImageURL *url.URL) (Manifest, error) {
	logger = logger.Session("fetching-image-manifest", lager.Data{"baseImageURL": baseImageURL})
	logger.Info("start")
	defer logger.Info("end")

	img, err := s.image(logger, baseImageURL)
	if err != nil {
		logger.Error("fetching-image-reference-failed", err)

		return Manifest{}, errorspkg.Wrap(err, "fetching image reference")
	}

	contents, mimeType, err := img.Manifest()
	if err != nil {
		logger.Error("fetching-manifest-failed", err)
		return Manifest{}, errorspkg.Wrap(err, "fetching manifest")
	}

	var manifest Manifest
	switch mimeType {
	case manifestpkg.DockerV2Schema1MediaType, manifestpkg.DockerV2Schema1SignedMediaType:
		logger.Debug("docker-image-version-2-schema-1")
		manifest, err = s.parseSchemaV1Manifest(logger, contents)

	case specsv1.MediaTypeImageManifest, manifestpkg.DockerV2Schema2MediaType:
		logger.Debug("docker-image-version-2-schema-2")
		manifest, err = s.parseSchemaV2Manifest(logger, contents)

	default:
		return Manifest{}, errors.New(fmt.Sprintf("unknown media type '%s'", mimeType))
	}

	return manifest, nil
}

func (s *DockerSource) Config(logger lager.Logger, baseImageURL *url.URL, manifest Manifest) (specsv1.Image, error) {
	logger = logger.Session("fetching-image-config", lager.Data{
		"baseImageURL": baseImageURL,
		"configDigest": manifest.ConfigCacheKey,
	})
	logger.Info("start")
	defer logger.Info("end")

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
		return specsv1.Image{}, fmt.Errorf("schema version not supported (%d)", manifest.SchemaVersion)
	}

	return config, nil
}

func (s *DockerSource) Blob(logger lager.Logger, baseImageURL *url.URL, digest string) (string, int64, error) {
	logrus.SetOutput(os.Stderr)
	logger = logger.Session("streaming-blob", lager.Data{
		"baseImageURL": baseImageURL,
		"digest":       digest,
	})
	logger.Info("start")
	defer logger.Info("end")

	imgSrc, err := s.imageSource(logger, baseImageURL)
	if err != nil {
		return "", 0, err
	}

	d := digestpkg.Digest(digest)
	blob, size, err := imgSrc.GetBlob(types.BlobInfo{Digest: d})
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
		return "", 0, fmt.Errorf("invalid checksum: layer is corrupted `%s`", digest)
	}

	return blobTempFile.Name(), size, nil
}

func (s *DockerSource) checkCheckSum(logger lager.Logger, data io.Reader, digest string) bool {
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

	if digestID != blobContentsSha {
		return false
	}

	return true
}

func (s *DockerSource) skipTLSValidation(baseImageURL *url.URL) bool {
	for _, trustedRegistry := range s.trustedRegistries {
		if baseImageURL.Host == trustedRegistry {
			return true
		}
	}

	return false
}

func (s *DockerSource) parseSchemaV1Manifest(logger lager.Logger, rawManifest []byte) (Manifest, error) {
	var dockerManifest SchemaV1Manifest
	if err := json.Unmarshal(rawManifest, &dockerManifest); err != nil {
		logger.Error("parsing-manifest-failed", err, lager.Data{"manifest": string(rawManifest)})
		return Manifest{}, fmt.Errorf("parsing manifest: %s", err)
	}

	manifest := Manifest{}
	for _, layer := range dockerManifest.FSLayers {
		manifest.Layers = append([]Layer{Layer{BlobID: layer["blobSum"]}}, manifest.Layers...)
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

func (s *DockerSource) parseSchemaV2Manifest(logger lager.Logger, rawManifest []byte) (Manifest, error) {
	var ociManifest specsv1.Manifest
	if err := json.Unmarshal(rawManifest, &ociManifest); err != nil {
		logger.Error("parsing-manifest-failed", err, lager.Data{"manifest": string(rawManifest)})
		return Manifest{}, fmt.Errorf("parsing manifest: %s", err)
	}

	manifest := Manifest{
		ConfigCacheKey: ociManifest.Config.Digest,
	}
	for _, layer := range ociManifest.Layers {
		manifest.Layers = append(manifest.Layers, Layer{BlobID: layer.Digest, Size: layer.Size})
	}

	manifest.SchemaVersion = 2
	return manifest, nil
}

func (s *DockerSource) parseSchemaV2Config(logger lager.Logger, baseImageURL *url.URL, configDigest string) (specsv1.Image, error) {
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
		return specsv1.Image{}, fmt.Errorf("parsing image config: %s", err)
	}

	return config, nil
}

func (s *DockerSource) parseSchemaV1Config(logger lager.Logger, manifest Manifest) (specsv1.Image, error) {
	if len(manifest.V1Compatibility) == 0 {
		logger.Error("v1-manifest-validation-failed", errors.New("v1compatibility has no layers"), lager.Data{"manifest": manifest})
		return specsv1.Image{}, errors.New("V1Compatibility is empty for the manifest")
	}

	var config specsv1.Image
	v1Config := manifest.V1Compatibility[len(manifest.V1Compatibility)-1]
	if err := json.Unmarshal([]byte(v1Config), &config); err != nil {
		logger.Error("parsing-manifest-v1-compatibility-failed", err)
		return specsv1.Image{}, fmt.Errorf("parsing manifest V1Compatibility: %s", err)
	}

	for _, rawHistory := range manifest.V1Compatibility {
		var v1Compatibility V1Compatibility
		if err := json.Unmarshal([]byte(rawHistory), &v1Compatibility); err != nil {
			logger.Error("parsing-manifest-v1-compatibility-failed", err)
			return specsv1.Image{}, fmt.Errorf("parsing manifest V1Compatibility: %s", err)
		}
		config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, "sha256:"+v1Compatibility.ID)
	}

	return config, nil
}

func (s *DockerSource) reference(logger lager.Logger, baseImageURL *url.URL) (types.ImageReference, error) {
	refString := "/"
	if baseImageURL.Host != "" {
		refString += "/" + baseImageURL.Host
	}
	refString += baseImageURL.Path

	logger.Debug("parsing-reference", lager.Data{"refString": refString})
	ref, err := docker.ParseReference(refString)
	if err != nil {
		return nil, fmt.Errorf("parsing url failed: %s", err)
	}

	return ref, nil
}

func (s *DockerSource) image(logger lager.Logger, baseImageURL *url.URL) (types.Image, error) {
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

func (s *DockerSource) imageSource(logger lager.Logger, baseImageURL *url.URL) (types.ImageSource, error) {
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
