package remote

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/Sirupsen/logrus"
	"github.com/containers/image/docker"
	"github.com/containers/image/types"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type DockerSource struct {
	trustedRegistries []string
}

func NewDockerSource(trustedRegistries []string) *DockerSource {
	return &DockerSource{
		trustedRegistries: trustedRegistries,
	}
}

func (s *DockerSource) Manifest(logger lager.Logger, imageURL *url.URL) (specsv1.Manifest, error) {
	logger = logger.Session("fetching-image-manifest", lager.Data{"imageURL": imageURL})
	logger.Info("start")
	defer logger.Info("end")

	img, err := s.preSteamedImage(logger, imageURL)
	if err != nil {
		return specsv1.Manifest{}, err
	}

	contents, _, err := img.Manifest()
	if err != nil {
		if strings.Contains(err.Error(), "error fetching manifest: status code:") {
			logger.Error("fetching-manifest-failed", err)
			return specsv1.Manifest{}, fmt.Errorf("image does not exist or you do not have permissions to see it: %s", err)
		}

		if strings.Contains(err.Error(), "malformed HTTP response") {
			logger.Error("fetching-manifest-failed", err)
			return specsv1.Manifest{}, fmt.Errorf("TLS validation of insecure registry failed: %s", err)
		}

		return specsv1.Manifest{}, err
	}

	var manifest specsv1.Manifest
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return specsv1.Manifest{}, fmt.Errorf("parsing manifest: %s", err)
	}

	return manifest, nil
}

func (s *DockerSource) Config(logger lager.Logger, imageURL *url.URL, configDigest string) (specsv1.Image, error) {
	logger = logger.Session("fetching-image-config", lager.Data{
		"imageURL":     imageURL,
		"configDigest": configDigest,
	})
	logger.Info("start")
	defer logger.Info("end")

	imgSrc, err := s.preSteamedImageSource(logger, imageURL)
	if err != nil {
		return specsv1.Image{}, err
	}

	stream, _, err := imgSrc.GetBlob(configDigest)
	if err != nil {
		if strings.Contains(err.Error(), "malformed HTTP response") {
			logger.Error("fetching-config-failed", err)
			return specsv1.Image{}, fmt.Errorf("TLS validation of insecure registry failed: %s", err)
		}
		return specsv1.Image{}, fmt.Errorf("fetching config blob: %s", err)
	}

	var config specsv1.Image
	if err := json.NewDecoder(stream).Decode(&config); err != nil {
		return specsv1.Image{}, fmt.Errorf("parsing image config: %s", err)
	}

	return config, nil
}

func (s *DockerSource) StreamBlob(logger lager.Logger, imageURL *url.URL, digest string) (io.ReadCloser, int64, error) {
	logrus.SetOutput(os.Stderr)
	logger = logger.Session("streaming-blob", lager.Data{
		"imageURL": imageURL,
		"digest":   digest,
	})
	logger.Info("start")
	defer logger.Info("end")

	imgSrc, err := s.preSteamedImageSource(logger, imageURL)
	if err != nil {
		return nil, 0, err
	}

	stream, size, err := imgSrc.GetBlob(digest)
	if err != nil {
		return nil, 0, err
	}
	logger.Debug("got-blob-stream", lager.Data{"size": size})

	tarStream, err := gzip.NewReader(stream)
	if err != nil {
		return nil, 0, fmt.Errorf("reading gzip: %s", err)
	}

	return tarStream, 0, nil
}

func (s *DockerSource) tlsVerify(imageURL *url.URL) bool {
	for _, trustedRegistry := range s.trustedRegistries {
		if imageURL.Host == trustedRegistry {
			return false
		}
	}

	return true
}

func (s *DockerSource) preSteamedReference(logger lager.Logger, imageURL *url.URL) (types.ImageReference, error) {
	refString := "/"
	if imageURL.Host != "" {
		refString += "/" + imageURL.Host
	}
	refString += imageURL.Path

	logger.Debug("parsing-reference", lager.Data{"refString": refString})
	ref, err := docker.ParseReference(refString)
	if err != nil {
		return nil, fmt.Errorf("parsing url failed: %s", err)
	}

	return ref, nil
}

func (s *DockerSource) preSteamedImage(logger lager.Logger, imageURL *url.URL) (types.Image, error) {
	ref, err := s.preSteamedReference(logger, imageURL)
	if err != nil {
		return nil, err
	}

	verifyTLS := s.tlsVerify(imageURL)
	logger.Debug("new-image", lager.Data{"verifyTLS": verifyTLS})
	img, err := ref.NewImage("", verifyTLS)
	if err != nil {
		return nil, fmt.Errorf("creating reference: %s", err)
	}

	return img, nil
}

func (s *DockerSource) preSteamedImageSource(logger lager.Logger, imageURL *url.URL) (types.ImageSource, error) {
	ref, err := s.preSteamedReference(logger, imageURL)
	if err != nil {
		return nil, err
	}

	verifyTLS := s.tlsVerify(imageURL)
	imgSrc, _ := ref.NewImageSource("", verifyTLS)
	if err != nil {
		return nil, fmt.Errorf("creating reference: %s", err)
	}
	logger.Debug("new-image", lager.Data{"verifyTLS": verifyTLS})

	return imgSrc, nil
}
