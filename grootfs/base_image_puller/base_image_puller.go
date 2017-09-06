package base_image_puller // import "code.cloudfoundry.org/grootfs/base_image_puller"

import (
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	errorspkg "github.com/pkg/errors"
)

const BaseImageReferenceFormat = "baseimage:%s"
const MetricsUnpackTimeName = "UnpackTime"
const MetricsDownloadTimeName = "DownloadTime"
const MetricsFailedUnpackTimeName = "FailedUnpackTime"

//go:generate counterfeiter . Fetcher
//go:generate counterfeiter . Unpacker
//go:generate counterfeiter . DependencyRegisterer
//go:generate counterfeiter . VolumeDriver

type UnpackSpec struct {
	Stream      io.ReadCloser `json:"-"`
	TargetPath  string
	UIDMappings []groot.IDMappingSpec
	GIDMappings []groot.IDMappingSpec
}

type LayerDigest struct {
	BlobID        string
	ChainID       string
	ParentChainID string
	Size          int64
}

type BaseImageInfo struct {
	LayersDigest []LayerDigest
	Config       specsv1.Image
}

type VolumeMeta struct {
	Size int64
}

type Fetcher interface {
	BaseImageInfo(logger lager.Logger, baseImageURL *url.URL) (BaseImageInfo, error)
	StreamBlob(logger lager.Logger, baseImageURL *url.URL, source string) (io.ReadCloser, int64, error)
}

type DependencyRegisterer interface {
	Register(id string, chainIDs []string) error
}

type UnpackOutput struct {
	BytesWritten    int64
	OpaqueWhiteouts []string
}

type Unpacker interface {
	Unpack(logger lager.Logger, spec UnpackSpec) (UnpackOutput, error)
}

type VolumeDriver interface {
	VolumePath(logger lager.Logger, id string) (string, error)
	CreateVolume(logger lager.Logger, parentID, id string) (string, error)
	DestroyVolume(logger lager.Logger, id string) error
	Volumes(logger lager.Logger) ([]string, error)
	MoveVolume(logger lager.Logger, from, to string) error
	WriteVolumeMeta(logger lager.Logger, id string, data VolumeMeta) error
	HandleOpaqueWhiteouts(logger lager.Logger, id string, opaqueWhiteouts []string) error
}

type BaseImagePuller struct {
	tarFetcher           Fetcher
	layerFetcher         Fetcher
	unpacker             Unpacker
	volumeDriver         VolumeDriver
	dependencyRegisterer DependencyRegisterer
	metricsEmitter       groot.MetricsEmitter
	locksmith            groot.Locksmith
}

func NewBaseImagePuller(tarFetcher, layerFetcher Fetcher, unpacker Unpacker, volumeDriver VolumeDriver, dependencyRegisterer DependencyRegisterer, metricsEmitter groot.MetricsEmitter, locksmith groot.Locksmith) *BaseImagePuller {
	return &BaseImagePuller{
		tarFetcher:           tarFetcher,
		layerFetcher:         layerFetcher,
		unpacker:             unpacker,
		volumeDriver:         volumeDriver,
		dependencyRegisterer: dependencyRegisterer,
		metricsEmitter:       metricsEmitter,
		locksmith:            locksmith,
	}
}

func (p *BaseImagePuller) Pull(logger lager.Logger, spec groot.BaseImageSpec) (groot.BaseImage, error) {
	logger = logger.Session("image-pulling", lager.Data{"spec": spec})
	logger.Info("starting")
	defer logger.Info("ending")

	baseImageInfo, err := p.fetcher(spec.BaseImageSrc).BaseImageInfo(logger, spec.BaseImageSrc)
	if err != nil {
		return groot.BaseImage{}, errorspkg.Wrap(err, "fetching list of digests")
	}
	logger.Debug("fetched-layers-digests", lager.Data{"digests": baseImageInfo.LayersDigest})

	if err = p.quotaExceeded(logger, baseImageInfo.LayersDigest, spec); err != nil {
		return groot.BaseImage{}, err
	}

	err = p.buildLayer(logger, len(baseImageInfo.LayersDigest)-1, baseImageInfo.LayersDigest, spec)
	if err != nil {
		return groot.BaseImage{}, err
	}
	chainIDs := p.chainIDs(baseImageInfo.LayersDigest)

	baseImageRefName := fmt.Sprintf(BaseImageReferenceFormat, spec.BaseImageSrc.String())
	if err := p.dependencyRegisterer.Register(baseImageRefName, chainIDs); err != nil {
		return groot.BaseImage{}, err
	}

	baseImage := groot.BaseImage{
		BaseImage: baseImageInfo.Config,
		ChainIDs:  chainIDs,
	}
	return baseImage, nil
}

func (p *BaseImagePuller) fetcher(baseImageURL *url.URL) Fetcher {
	if baseImageURL.Scheme == "" {
		return p.tarFetcher
	}

	return p.layerFetcher
}

func (p *BaseImagePuller) quotaExceeded(logger lager.Logger, layersDigest []LayerDigest, spec groot.BaseImageSpec) error {
	if spec.ExcludeBaseImageFromQuota || spec.DiskLimit == 0 {
		return nil
	}

	totalSize := p.layersSize(layersDigest)
	if totalSize > spec.DiskLimit {
		err := errorspkg.Errorf("layers exceed disk quota %d/%d bytes", totalSize, spec.DiskLimit)
		logger.Error("blob-manifest-size-check-failed", err, lager.Data{
			"totalSize":                 totalSize,
			"diskLimit":                 spec.DiskLimit,
			"excludeBaseImageFromQuota": spec.ExcludeBaseImageFromQuota,
		})
		return err
	}

	return nil
}

func (p *BaseImagePuller) chainIDs(layersDigest []LayerDigest) []string {
	chainIDs := []string{}
	for _, layerDigest := range layersDigest {
		chainIDs = append(chainIDs, layerDigest.ChainID)
	}
	return chainIDs
}

func (p *BaseImagePuller) volumeExists(logger lager.Logger, chainID string) bool {
	volumePath, err := p.volumeDriver.VolumePath(logger, chainID)
	if err == nil {
		logger.Debug("volume-exists", lager.Data{
			"volumePath": volumePath,
		})

		return true
	}

	return false
}

func (p *BaseImagePuller) buildLayer(logger lager.Logger, index int, layersDigests []LayerDigest, spec groot.BaseImageSpec) error {
	if index < 0 {
		return nil
	}

	digest := layersDigests[index]
	logger = logger.Session("build-layer", lager.Data{
		"blobID":        digest.BlobID,
		"chainID":       digest.ChainID,
		"parentChainID": digest.ParentChainID,
	})
	if p.volumeExists(logger, digest.ChainID) {
		return nil
	}

	lockFile, err := p.locksmith.Lock(digest.ChainID)
	if err != nil {
		return errorspkg.Wrap(err, "acquiring lock")
	}
	defer p.locksmith.Unlock(lockFile)

	if p.volumeExists(logger, digest.ChainID) {
		return nil
	}

	downloadChan := make(chan downloadReturn, 1)
	go p.downloadLayer(logger, spec, digest, downloadChan)

	if err := p.buildLayer(logger, index-1, layersDigests, spec); err != nil {
		return err
	}

	downloadResult := <-downloadChan
	if downloadResult.Err != nil {
		return downloadResult.Err
	}

	defer downloadResult.Stream.Close()

	return p.unpackLayer(logger, digest, spec, downloadResult.Stream)
}

type downloadReturn struct {
	Stream io.ReadCloser
	Err    error
}

func (p *BaseImagePuller) downloadLayer(logger lager.Logger, spec groot.BaseImageSpec, digest LayerDigest, downloadChan chan downloadReturn) {
	logger = logger.Session("downloading-layer", lager.Data{"LayerDigest": digest})
	logger.Debug("starting")
	defer logger.Debug("ending")
	defer p.metricsEmitter.TryEmitDurationFrom(logger, MetricsDownloadTimeName, time.Now())

	stream, size, err := p.fetcher(spec.BaseImageSrc).StreamBlob(logger, spec.BaseImageSrc, digest.BlobID)
	if err != nil {
		err = errorspkg.Wrapf(err, "streaming blob `%s`", digest.BlobID)
	}

	logger.Debug("got-stream-for-blob", lager.Data{
		"size":                      size,
		"diskLimit":                 spec.DiskLimit,
		"excludeBaseImageFromQuota": spec.ExcludeBaseImageFromQuota,
	})

	downloadChan <- downloadReturn{Stream: stream, Err: err}
}

func (p *BaseImagePuller) unpackLayer(logger lager.Logger, digest LayerDigest, spec groot.BaseImageSpec, stream io.ReadCloser) error {
	logger = logger.Session("unpacking-layer", lager.Data{"LayerDigest": digest})
	logger.Debug("starting")
	defer logger.Debug("ending")

	tempVolumeName, volumePath, err := p.createTemporaryVolumeDirectory(logger, digest, spec)
	if err != nil {
		return err
	}

	unpackSpec := UnpackSpec{
		TargetPath:  volumePath,
		Stream:      stream,
		UIDMappings: spec.UIDMappings,
		GIDMappings: spec.GIDMappings,
	}

	volSize, err := p.unpackLayerToTemporaryDirectory(logger, unpackSpec, digest)
	if err != nil {
		return err
	}

	return p.finalizeVolume(logger, tempVolumeName, volumePath, digest.ChainID, volSize)
}

func (p *BaseImagePuller) createTemporaryVolumeDirectory(logger lager.Logger, digest LayerDigest, spec groot.BaseImageSpec) (string, string, error) {
	tempVolumeName := fmt.Sprintf("%s-incomplete-%d-%d", digest.ChainID, time.Now().UnixNano(), rand.Int())
	volumePath, err := p.volumeDriver.CreateVolume(logger,
		digest.ParentChainID,
		tempVolumeName,
	)
	if err != nil {
		return "", "", errorspkg.Wrapf(err, "creating volume for layer `%s`", digest.BlobID)
	}
	logger.Debug("volume-created", lager.Data{"volumePath": volumePath})

	if spec.OwnerUID != 0 || spec.OwnerGID != 0 {
		err = os.Chown(volumePath, spec.OwnerUID, spec.OwnerGID)
		if err != nil {
			return "", "", errorspkg.Wrapf(err, "changing volume ownership to %d:%d", spec.OwnerUID, spec.OwnerGID)
		}
	}

	return tempVolumeName, volumePath, nil
}

func (p *BaseImagePuller) unpackLayerToTemporaryDirectory(logger lager.Logger, unpackSpec UnpackSpec, digest LayerDigest) (volSize int64, err error) {
	defer p.metricsEmitter.TryEmitDurationFrom(logger, MetricsUnpackTimeName, time.Now())
	var unpackOutput UnpackOutput

	if unpackOutput, err = p.unpacker.Unpack(logger, unpackSpec); err != nil {
		if errD := p.volumeDriver.DestroyVolume(logger, digest.ChainID); errD != nil {
			logger.Error("volume-cleanup-failed", errD)
		}
		return 0, errorspkg.Wrapf(err, "unpacking layer `%s`", digest.BlobID)
	}

	if err := p.volumeDriver.HandleOpaqueWhiteouts(logger, path.Base(unpackSpec.TargetPath), unpackOutput.OpaqueWhiteouts); err != nil {
		logger.Error("handling-opaque-whiteouts", err)
		return 0, errorspkg.Wrap(err, "handling opaque whiteouts")
	}

	logger.Debug("layer-unpacked")
	return unpackOutput.BytesWritten, nil
}

func (p *BaseImagePuller) finalizeVolume(logger lager.Logger, tempVolumeName, volumePath, chainID string, volSize int64) error {
	finalVolumePath := strings.Replace(volumePath, tempVolumeName, chainID, 1)
	if err := p.volumeDriver.MoveVolume(logger, volumePath, finalVolumePath); err != nil {
		return errorspkg.Wrapf(err, "failed to move volume to its final location")
	}

	if err := p.volumeDriver.WriteVolumeMeta(logger, chainID, VolumeMeta{Size: volSize}); err != nil {
		return errorspkg.Wrapf(err, "writing volume `%s` metadata", chainID)
	}
	return nil
}

func (p *BaseImagePuller) layersSize(layerDigests []LayerDigest) int64 {
	var totalSize int64
	for _, digest := range layerDigests {
		totalSize += digest.Size
	}
	return totalSize
}
