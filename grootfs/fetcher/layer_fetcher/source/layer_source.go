package source // import "code.cloudfoundry.org/grootfs/fetcher/layer_fetcher/source"

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/url"
	"os"
	"strings"

	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager/v3"
	_ "github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/image"
	manifestpkg "github.com/containers/image/v5/manifest"
	_ "github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/pkg/blobinfocache/none"
	"github.com/containers/image/v5/types"
	digestpkg "github.com/opencontainers/go-digest"
	errorspkg "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	MAX_DOCKER_RETRIES = 3
	UNKNOWN_LAYER_SIZE = -1
)

//go:generate counterfeiter . ImageSourceCreator
type ImageSourceCreator func(logger lager.Logger, systemContext types.SystemContext, baseImageURL *url.URL) (types.ImageSource, error)

type LayerSource struct {
	skipOCILayerValidation bool
	systemContext          types.SystemContext
	baseImageURL           *url.URL
	// imageSource needs to be a singleton that is initialised on demand in createImageSource. DO NOT use the field directly, use getImageSource instead
	imageSource              types.ImageSource
	imageQuota               int64
	skipImageQuotaValidation bool
	imageSourceCreator       ImageSourceCreator
}

func NewLayerSource(systemContext types.SystemContext, skipOCILayerValidation, skipImageQuotaValidation bool, diskLimit int64, baseImageURL *url.URL, imageSourceCreator ImageSourceCreator) LayerSource {
	return LayerSource{
		systemContext:            systemContext,
		skipOCILayerValidation:   skipOCILayerValidation,
		baseImageURL:             baseImageURL,
		imageQuota:               diskLimit,
		skipImageQuotaValidation: skipImageQuotaValidation,
		imageSourceCreator:       imageSourceCreator,
	}
}

func (s *LayerSource) Manifest(logger lager.Logger) (types.Image, error) {
	logger = logger.Session("fetching-image-manifest", lager.Data{"baseImageURL": s.baseImageURL})
	logger.Info("starting")
	defer logger.Info("ending")

	img, err := s.getImageWithRetries(logger)
	if err != nil {
		logger.Error("fetching-image-reference-failed", err)
		return nil, errorspkg.Wrap(err, "fetching image reference")
	}

	img, err = s.convertImage(logger, img)
	if err != nil {
		logger.Error("converting-image-failed", err)
		return nil, err
	}

	for i := 0; i < MAX_DOCKER_RETRIES; i++ {
		logger.Debug("attempt-get-config", lager.Data{"attempt": i + 1})
		_, e := img.ConfigBlob(context.TODO())
		if e == nil {
			return img, nil
		}

		logger.Error("fetching-image-config-failed", e, lager.Data{"attempt": i + 1})
		err = e
	}

	return nil, errorspkg.Wrap(err, "fetching image configuration")
}

func (s *LayerSource) Blob(logger lager.Logger, layerInfo groot.LayerInfo) (string, int64, error) {
	logrus.SetOutput(os.Stderr)
	logger = logger.Session("streaming-blob", lager.Data{
		"baseImageURL":             s.baseImageURL,
		"digest":                   layerInfo.BlobID,
		"imageQuota":               s.imageQuota,
		"skipImageQuotaValidation": s.skipImageQuotaValidation,
	})
	logger.Info("starting")
	defer logger.Info("ending")

	imgSrc, err := s.getImageSource(logger)
	if err != nil {
		return "", 0, err
	}

	blobInfo := types.BlobInfo{
		Digest: digestpkg.Digest(layerInfo.BlobID),
		URLs:   layerInfo.URLs,
	}

	blob, reportedSize, err := s.getBlobWithRetries(logger, imgSrc, blobInfo)
	if err != nil {
		return "", 0, err
	}
	defer blob.Close()

	countingBlobReader := NewCountingReader(blob)
	logger.Debug("got-blob-stream", lager.Data{"digest": layerInfo.BlobID, "reportedSize": reportedSize, "mediaType": layerInfo.MediaType})

	if err = s.validateLayerSize(layerInfo, reportedSize); err != nil {
		return "", 0, errorspkg.Wrap(err, "validating reported blob size")
	}

	blobTempFile, err := os.CreateTemp("", fmt.Sprintf("blob-%s", strings.Replace(layerInfo.BlobID, ":", "-", -1)))
	if err != nil {
		return "", 0, err
	}

	defer func() {
		blobTempFile.Close()

		if err != nil {
			os.Remove(blobTempFile.Name())
		}
	}()

	blobIDHash := sha256.New()
	digestReader := io.NopCloser(io.TeeReader(countingBlobReader, blobIDHash))
	if layerInfo.MediaType == "" || strings.Contains(layerInfo.MediaType, "gzip") {
		logger.Debug("uncompressing-blob")

		digestReader, err = gzip.NewReader(digestReader)
		if err != nil {
			return "", 0, errorspkg.Wrapf(err, "expected blob to be of type %s", layerInfo.MediaType)
		}
		defer digestReader.Close()

	}

	if s.shouldEnforceImageQuotaValidation() {
		digestReader = layer_fetcher.NewQuotaedReader(digestReader, s.imageQuota, "uncompressed layer size exceeds quota")
	}

	diffIDHash := sha256.New()
	digestReader = io.NopCloser(io.TeeReader(digestReader, diffIDHash))

	uncompressedSize, err := io.Copy(blobTempFile, digestReader)
	if err != nil {
		logger.Error("writing-blob-to-file", err)
		return "", 0, errorspkg.Wrap(err, "writing blob to tempfile")
	}

	actualSize := countingBlobReader.GetBytesRead()
	if err = s.validateLayerSize(layerInfo, actualSize); err != nil {
		return "", 0, errorspkg.Wrap(err, "validating actual blob size")
	}

	blobIDHex := strings.Split(layerInfo.BlobID, ":")[1]
	if err = s.checkCheckSum(logger, blobIDHash, blobIDHex); err != nil {
		return "", 0, errorspkg.Wrap(err, "layerID digest mismatch")
	}

	if err = s.checkCheckSum(logger, diffIDHash, layerInfo.DiffID); err != nil {
		return "", 0, errorspkg.Wrap(err, "diffID digest mismatch")
	}

	s.imageQuota -= uncompressedSize

	return blobTempFile.Name(), actualSize, nil
}

func (s *LayerSource) shouldEnforceImageQuotaValidation() bool {
	return !s.skipImageQuotaValidation
}

func (s *LayerSource) validateLayerSize(layerInfo groot.LayerInfo, size int64) error {
	if s.skipOCILayerValidation || isV1Image(layerInfo) || size == UNKNOWN_LAYER_SIZE || layerInfo.Size == size {
		return nil
	}

	return fmt.Errorf("layer size is different from the value in the manifest; expected: %d, actual %d", layerInfo.Size, size)
}

func isV1Image(layerInfo groot.LayerInfo) bool {
	return layerInfo.Size == -1
}

func (s *LayerSource) Close() error {
	if s.imageSource != nil {
		return s.imageSource.Close()
	}
	return nil
}

func (s *LayerSource) getBlobWithRetries(logger lager.Logger, imgSrc types.ImageSource, blobInfo types.BlobInfo) (io.ReadCloser, int64, error) {
	var err error
	for i := 0; i < MAX_DOCKER_RETRIES; i++ {
		logger.Debug(fmt.Sprintf("attempt-get-blob-%d", i+1))
		blob, size, e := imgSrc.GetBlob(context.TODO(), blobInfo, none.NoCache)
		if e == nil {
			logger.Debug("attempt-get-blob-success")
			return blob, size, nil
		}
		err = e
		logger.Error("attempt-get-blob-failed", err)
	}

	return nil, 0, err
}

func (s *LayerSource) checkCheckSum(logger lager.Logger, hash hash.Hash, digest string) error {
	if s.skipOCILayerValidation && s.baseImageURL.Scheme == "oci" {
		return nil
	}

	blobContentsSha := hex.EncodeToString(hash.Sum(nil))
	logger.Debug("checking-checksum", lager.Data{
		"digestIDChecksum":   digest,
		"downloadedChecksum": blobContentsSha,
	})
	if digest != blobContentsSha {
		return errorspkg.Errorf("expected: %s, actual: %s", digest, blobContentsSha)
	}

	return nil
}

func (s *LayerSource) getImageWithRetries(logger lager.Logger) (types.Image, error) {
	var imgErr error
	var img types.Image
	for i := 0; i < MAX_DOCKER_RETRIES; i++ {
		logger.Debug(fmt.Sprintf("attempt-get-image-%d", i+1))

		imageSource, err := s.getImageSource(logger)
		if err == nil {
			img, err = image.FromUnparsedImage(context.TODO(), &s.systemContext, image.UnparsedInstance(imageSource, nil))
			if err == nil {
				logger.Debug("attempt-get-image-success")
				return img, nil
			}
		}
		imgErr = err
	}

	return nil, errorspkg.Wrap(imgErr, "creating image")
}

func (s *LayerSource) getImageSource(logger lager.Logger) (types.ImageSource, error) {
	if s.imageSource == nil {
		var err error
		s.imageSource, err = s.imageSourceCreator(logger, s.systemContext, s.baseImageURL)
		if err != nil {
			return nil, err
		}
	}

	return s.imageSource, nil
}

func (s *LayerSource) convertImage(logger lager.Logger, originalImage types.Image) (types.Image, error) {
	_, mimetype, err := originalImage.Manifest(context.TODO())
	if err != nil {
		return nil, err
	}

	if mimetype != manifestpkg.DockerV2Schema1MediaType && mimetype != manifestpkg.DockerV2Schema1SignedMediaType {
		return originalImage, nil
	}

	logger = logger.Session("convert-schema-V1-image")
	logger.Info("starting")
	defer logger.Info("ending")

	imgSrc, err := s.getImageSource(logger)
	if err != nil {
		return nil, err
	}

	diffIDs := []digestpkg.Digest{}
	for _, layer := range originalImage.LayerInfos() {
		diffID, err := s.v1DiffID(logger, layer, imgSrc)
		if err != nil {
			return nil, errorspkg.Wrap(err, "converting V1 schema failed")
		}
		diffIDs = append(diffIDs, diffID)
	}

	options := types.ManifestUpdateOptions{
		ManifestMIMEType: manifestpkg.DockerV2Schema2MediaType,
		InformationOnly: types.ManifestUpdateInformation{
			LayerDiffIDs: diffIDs,
			LayerInfos:   originalImage.LayerInfos(),
		},
	}

	return originalImage.UpdatedImage(context.TODO(), options)
}

func (s *LayerSource) v1DiffID(logger lager.Logger, layer types.BlobInfo, imgSrc types.ImageSource) (digestpkg.Digest, error) {
	blob, _, err := s.getBlobWithRetries(logger, imgSrc, layer)
	if err != nil {
		return "", errorspkg.Wrap(err, "fetching V1 layer blob")
	}
	defer blob.Close()

	gzipReader, err := gzip.NewReader(blob)
	if err != nil {
		return "", errorspkg.Wrap(err, "creating reader for V1 layer blob")
	}

	data, err := io.ReadAll(gzipReader)
	if err != nil {
		return "", errorspkg.Wrap(err, "reading V1 layer blob")
	}
	sha := sha256.Sum256(data)

	return digestpkg.NewDigestFromHex("sha256", hex.EncodeToString(sha[:])), nil
}
