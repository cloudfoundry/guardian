package source // import "code.cloudfoundry.org/grootfs/fetcher/layer_fetcher/source"

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"code.cloudfoundry.org/lager"
	_ "github.com/containers/image/docker"
	manifestpkg "github.com/containers/image/manifest"
	_ "github.com/containers/image/oci/layout"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	digestpkg "github.com/opencontainers/go-digest"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	errorspkg "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const MAX_DOCKER_RETRIES = 3

type LayerSource struct {
	skipOCIChecksumValidation bool
	systemContext             types.SystemContext
}

func NewLayerSource(systemContext types.SystemContext, skipOCIChecksumValidation bool) LayerSource {
	return LayerSource{
		systemContext:             systemContext,
		skipOCIChecksumValidation: skipOCIChecksumValidation,
	}
}

func (s *LayerSource) Manifest(logger lager.Logger, baseImageURL *url.URL) (types.Image, error) {
	logger = logger.Session("fetching-image-manifest", lager.Data{"baseImageURL": baseImageURL})
	logger.Info("starting")
	defer logger.Info("ending")

	img, err := s.getImageWithRetries(logger, baseImageURL)
	if err != nil {
		logger.Error("fetching-image-reference-failed", err)
		return nil, errorspkg.Wrap(err, "fetching image reference")
	}

	img, err = s.convertImage(logger, img, baseImageURL)
	if err != nil {
		logger.Error("converting-image-failed", err)
		return nil, err
	}

	for i := 0; i < MAX_DOCKER_RETRIES; i++ {
		logger.Debug("attempt-get-config", lager.Data{"attempt": i + 1})
		_, e := img.ConfigBlob()
		if e == nil {
			return img, nil
		}

		logger.Error("fetching-image-config-failed", e, lager.Data{"attempt": i + 1})
		err = e
	}

	return nil, errorspkg.Wrap(err, "fetching image configuration")
}

func (s *LayerSource) Blob(logger lager.Logger, baseImageURL *url.URL, digest string, layersUrls []string) (string, int64, error) {
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

	blobInfo := types.BlobInfo{
		Digest: digestpkg.Digest(digest),
		URLs:   layersUrls,
	}

	blob, size, err := s.getBlobWithRetries(logger, imgSrc, blobInfo)
	if err != nil {
		return "", 0, err
	}
	logger.Debug("got-blob-stream", lager.Data{"digest": digest, "size": size})

	blobTempFile, err := ioutil.TempFile("", fmt.Sprintf("blob-%s", digest))
	if err != nil {
		return "", 0, err
	}
	defer func() {
		blob.Close()
		blobTempFile.Close()
	}()

	hash := sha256.New()
	blobWriter := io.MultiWriter(blobTempFile, hash)
	if _, err := io.Copy(blobWriter, blob); err != nil {
		logger.Error("writing-blob-to-file", err)
		return "", 0, errorspkg.Wrap(err, "writing blob to tempfile")
	}

	if !s.checkCheckSum(logger, hash, digest, baseImageURL.Scheme) {
		return "", 0, errorspkg.Errorf("invalid checksum: layer is corrupted `%s`", digest)
	}

	return blobTempFile.Name(), size, nil
}

func (s *LayerSource) getBlobWithRetries(logger lager.Logger, imgSrc types.ImageSource, blobInfo types.BlobInfo) (io.ReadCloser, int64, error) {
	var err error
	for i := 0; i < MAX_DOCKER_RETRIES; i++ {
		logger.Debug(fmt.Sprintf("attempt-get-blob-%d", i+1))
		blob, size, e := imgSrc.GetBlob(blobInfo)
		if e == nil {
			logger.Debug("attempt-get-blob-success")
			return blob, size, nil
		}
		err = e
		logger.Error("attempt-get-blob-failed", err)
	}

	return nil, 0, err
}

func (s *LayerSource) checkCheckSum(logger lager.Logger, hash hash.Hash, digest string, scheme string) bool {
	if s.skipOCIChecksumValidation && scheme == "oci" {
		return true
	}

	digestID := strings.Split(digest, ":")[1]
	blobContentsSha := hex.EncodeToString(hash.Sum(nil))
	logger.Debug("checking-checksum", lager.Data{
		"digestIDChecksum":   digestID,
		"downloadedChecksum": blobContentsSha,
	})
	return digestID == blobContentsSha
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

func (s *LayerSource) getImageWithRetries(logger lager.Logger, baseImageURL *url.URL) (types.Image, error) {
	ref, err := s.reference(logger, baseImageURL)
	if err != nil {
		return nil, err
	}

	var imgErr error
	for i := 0; i < MAX_DOCKER_RETRIES; i++ {
		logger.Debug(fmt.Sprintf("attempt-get-image-%d", i+1))

		img, e := ref.NewImage(&s.systemContext)
		if e == nil {
			logger.Debug("attempt-get-image-success")
			return img, nil
		}
		imgErr = e
	}

	return nil, errorspkg.Wrap(imgErr, "creating image")
}

func (s *LayerSource) imageSource(logger lager.Logger, baseImageURL *url.URL) (types.ImageSource, error) {
	ref, err := s.reference(logger, baseImageURL)
	if err != nil {
		return nil, err
	}

	imgSrc, err := ref.NewImageSource(&s.systemContext)
	if err != nil {
		return nil, errorspkg.Wrap(err, "creating image source")
	}

	return imgSrc, nil
}

func (s *LayerSource) convertImage(logger lager.Logger, originalImage types.Image, baseImageURL *url.URL) (types.Image, error) {
	_, mimetype, err := originalImage.Manifest()
	if err != nil {
		return nil, err
	}

	if mimetype != manifestpkg.DockerV2Schema1MediaType && mimetype != manifestpkg.DockerV2Schema1SignedMediaType {
		return originalImage, nil
	}

	logger = logger.Session("convert-schema-V1-image")
	logger.Info("starting")
	defer logger.Info("ending")

	imgSrc, err := s.imageSource(logger, baseImageURL)
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
		},
	}

	return originalImage.UpdatedImage(options)
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

	data, err := ioutil.ReadAll(gzipReader)
	if err != nil {
		return "", errorspkg.Wrap(err, "reading V1 layer blob")
	}
	sha := sha256.Sum256(data)

	return digestpkg.NewDigestFromHex("sha256", hex.EncodeToString(sha[:])), nil
}

func preferedMediaTypes() []string {
	return []string{
		specsv1.MediaTypeImageManifest,
		manifestpkg.DockerV2Schema2MediaType,
	}
}
