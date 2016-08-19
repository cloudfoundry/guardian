package fetcher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/containers/image/docker"
	"github.com/containers/image/types"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/lager"
)

type Fetcher struct {
	cachePath string
}

func NewFetcher(cachePath string) *Fetcher {
	return &Fetcher{
		cachePath: cachePath,
	}
}

func (f *Fetcher) LayersDigest(logger lager.Logger, imageURL *url.URL) ([]cloner.LayerDigest, error) {
	logger = logger.Session("layers-digest", lager.Data{"imageURL": imageURL})
	logger.Info("start")
	logger.Info("end")

	logger.Debug("parsing-reference")
	refString := "/"
	if imageURL.Host != "" {
		refString += "/" + imageURL.Host
	}
	refString += imageURL.Path

	ref, err := docker.ParseReference(refString)
	if err != nil {
		return nil, fmt.Errorf("parsing url failed: %s", err)
	}

	logger.Debug("parsing-image-metadata")
	img, err := ref.NewImage("", false)
	if err != nil {
		return nil, fmt.Errorf("creating an image: %s", err)
	}

	logger.Debug("parsing-image-source")
	imgSrc, err := ref.NewImageSource("", false)
	if err != nil {
		return nil, fmt.Errorf("creating image source: %s", err)
	}

	logger.Debug("inspecting-image")
	inspectInfo, err := img.Inspect()
	if err != nil {
		return nil, fmt.Errorf("inspecting image: %s", err)
	}

	logger.Debug("fetching-image-manifest")
	imgManifest, err := f.parsedManifest(img)
	if err != nil {
		return nil, fmt.Errorf("parsing manifest: %s", err)
	}

	logger.Debug("fetching-image-config")
	imgConfig, err := f.parsedConfig(imgSrc, imgManifest)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %s", err)
	}

	return f.createLayersDigest(imgConfig, inspectInfo.Layers), nil
}

func (f *Fetcher) Streamer(logger lager.Logger, imageURL *url.URL) (cloner.Streamer, error) {
	logger = logger.Session("streaming", lager.Data{"imageURL": imageURL})
	logger.Info("start")
	defer logger.Info("end")

	logger.Debug("parsing-reference")
	ref, err := docker.ParseReference("/" + imageURL.Path)
	if err != nil {
		return nil, fmt.Errorf("parsing url failed: %s", err)
	}

	logger.Debug("parsing-image-source")
	imgSrc, err := ref.NewImageSource("", true)
	if err != nil {
		return nil, fmt.Errorf("creating image source: %s", err)
	}

	remoteStreamer := NewRemoteStreamer(imgSrc)
	return NewCachedStreamer(f.cachePath, remoteStreamer), nil
}

type imageManifest struct {
	Config map[string]interface{} `json:"config"`
}

type imageConfig struct {
	Rootfs imageRootfs `json:"rootfs"`
}

type imageRootfs struct {
	DiffIds []string `json:"diff_ids"`
}

func (f *Fetcher) parsedManifest(img types.Image) (imageManifest, error) {
	manifest := new(imageManifest)
	manifestData, _, err := img.Manifest()
	if err != nil {
		return *manifest, fmt.Errorf("fetching manifest: %s", err)
	}

	if err := json.Unmarshal(manifestData, manifest); err != nil {
		return *manifest, fmt.Errorf("parsing image manifest: %s", err)
	}

	return *manifest, nil
}

func (f *Fetcher) parsedConfig(imgSrc types.ImageSource, manifest imageManifest) (imageConfig, error) {
	imageConfig := new(imageConfig)
	configReader, _, err := imgSrc.GetBlob(manifest.Config["digest"].(string))
	if err != nil {
		return *imageConfig, fmt.Errorf("fetching config blob: %s", err)
	}

	if err := json.NewDecoder(configReader).Decode(imageConfig); err != nil {
		return *imageConfig, fmt.Errorf("parsing image config: %s", err)
	}

	return *imageConfig, nil
}

func (f *Fetcher) createLayersDigest(config imageConfig, layers []string) []cloner.LayerDigest {
	layersDigest := []cloner.LayerDigest{}
	var parentChainID string
	for i, blobID := range layers {
		if i == 0 {
			parentChainID = ""
		}

		diffID := config.Rootfs.DiffIds[i]
		chainID := f.chainID(diffID, parentChainID)

		layersDigest = append(layersDigest, cloner.LayerDigest{
			BlobID:        blobID,
			DiffID:        diffID,
			ChainID:       chainID,
			ParentChainID: parentChainID,
		})

		parentChainID = chainID
	}
	return layersDigest
}

func (f *Fetcher) chainID(diffID string, parentChainID string) string {
	diffID = strings.Split(diffID, ":")[1]
	chainID := diffID

	if parentChainID != "" {
		parentChainID = strings.Split(parentChainID, ":")[1]
		chainIDSha := sha256.Sum256([]byte(fmt.Sprintf("%s %s", parentChainID, diffID)))
		chainID = hex.EncodeToString(chainIDSha[:32])
	}

	return "sha256:" + chainID
}
