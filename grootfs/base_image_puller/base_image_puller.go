package base_image_puller // import "code.cloudfoundry.org/grootfs/base_image_puller"

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

const MetricsUnpackTimeName = "UnpackTime"
const MetricsDownloadTimeName = "DownloadTime"

//go:generate counterfeiter . Fetcher
//go:generate counterfeiter . Unpacker
//go:generate counterfeiter . DependencyRegisterer
//go:generate counterfeiter . VolumeDriver

type UnpackSpec struct {
	Stream        io.ReadCloser `json:"-"`
	TargetPath    string
	UIDMappings   []groot.IDMappingSpec
	GIDMappings   []groot.IDMappingSpec
	BaseDirectory string
}

type VolumeMeta struct {
	Size int64
}

type Fetcher interface {
	BaseImageInfo(logger lager.Logger) (groot.BaseImageInfo, error)
	StreamBlob(logger lager.Logger, layerInfo groot.LayerInfo) (io.ReadCloser, int64, error)
	Close() error
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
	fetcher        Fetcher
	unpacker       Unpacker
	volumeDriver   VolumeDriver
	metricsEmitter groot.MetricsEmitter
	locksmith      groot.Locksmith
}

func NewBaseImagePuller(fetcher Fetcher, unpacker Unpacker, volumeDriver VolumeDriver, metricsEmitter groot.MetricsEmitter, locksmith groot.Locksmith) *BaseImagePuller {
	return &BaseImagePuller{
		fetcher:        fetcher,
		unpacker:       unpacker,
		volumeDriver:   volumeDriver,
		metricsEmitter: metricsEmitter,
		locksmith:      locksmith,
	}
}

func (p *BaseImagePuller) FetchBaseImageInfo(logger lager.Logger) (groot.BaseImageInfo, error) {
	logger = logger.Session("fetching-image-info")
	logger.Info("starting")
	defer logger.Info("ending")

	return p.fetcher.BaseImageInfo(logger)
}

func (p *BaseImagePuller) Pull(logger lager.Logger, baseImageInfo groot.BaseImageInfo, spec groot.BaseImageSpec) error {
	logger = logger.Session("pulling-image-layers", lager.Data{"spec": spec})
	logger.Info("starting")
	defer logger.Info("ending")

	if err := p.quotaExceeded(logger, baseImageInfo.LayerInfos, spec); err != nil {
		return err
	}

	return p.buildLayer(logger, len(baseImageInfo.LayerInfos)-1, baseImageInfo.LayerInfos, spec)
}

func (p *BaseImagePuller) quotaExceeded(logger lager.Logger, layerInfos []groot.LayerInfo, spec groot.BaseImageSpec) error {
	if spec.ExcludeBaseImageFromQuota || spec.DiskLimit == 0 {
		return nil
	}

	totalSize := p.layersSize(layerInfos)
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

func (p *BaseImagePuller) buildLayer(logger lager.Logger, index int, layerInfos []groot.LayerInfo, spec groot.BaseImageSpec) error {
	if index < 0 {
		return nil
	}

	layerInfo := layerInfos[index]
	logger = logger.Session("build-layer", lager.Data{
		"blobID":        layerInfo.BlobID,
		"chainID":       layerInfo.ChainID,
		"parentChainID": layerInfo.ParentChainID,
	})
	if p.volumeExists(logger, layerInfo.ChainID) {
		return nil
	}

	lockFile, err := p.locksmith.Lock(layerInfo.ChainID)
	if err != nil {
		return errorspkg.Wrap(err, "acquiring lock")
	}
	defer p.locksmith.Unlock(lockFile)

	if p.volumeExists(logger, layerInfo.ChainID) {
		return nil
	}

	downloadChan := make(chan downloadReturn, 1)
	go p.downloadLayer(logger, layerInfo, downloadChan)

	if err := p.buildLayer(logger, index-1, layerInfos, spec); err != nil {
		return err
	}

	downloadResult := <-downloadChan
	if downloadResult.Err != nil {
		return downloadResult.Err
	}

	defer downloadResult.Stream.Close()

	var parentLayerInfo groot.LayerInfo
	if index > 0 {
		parentLayerInfo = layerInfos[index-1]
	}
	return p.unpackLayer(logger, layerInfo, parentLayerInfo, spec, downloadResult.Stream)
}

type downloadReturn struct {
	Stream io.ReadCloser
	Err    error
}

func (p *BaseImagePuller) downloadLayer(logger lager.Logger, layerInfo groot.LayerInfo, downloadChan chan downloadReturn) {
	logger = logger.Session("downloading-layer", lager.Data{"LayerInfo": layerInfo})
	logger.Debug("starting")
	defer logger.Debug("ending")
	defer p.metricsEmitter.TryEmitDurationFrom(logger, MetricsDownloadTimeName, time.Now())

	stream, size, err := p.fetcher.StreamBlob(logger, layerInfo)
	if err != nil {
		err = errorspkg.Wrapf(err, "streaming blob `%s`", layerInfo.BlobID)
	}

	logger.Debug("got-stream-for-blob", lager.Data{"size": size})

	downloadChan <- downloadReturn{Stream: stream, Err: err}
}

func (p *BaseImagePuller) unpackLayer(logger lager.Logger, layerInfo, parentLayerInfo groot.LayerInfo, spec groot.BaseImageSpec, stream io.ReadCloser) error {
	logger = logger.Session("unpacking-layer", lager.Data{"LayerInfo": layerInfo})
	logger.Debug("starting")
	defer logger.Debug("ending")

	tempVolumeName, volumePath, err := p.createTemporaryVolumeDirectory(logger, layerInfo, spec)
	if err != nil {
		return err
	}

	unpackSpec := UnpackSpec{
		TargetPath:    volumePath,
		Stream:        stream,
		UIDMappings:   spec.UIDMappings,
		GIDMappings:   spec.GIDMappings,
		BaseDirectory: layerInfo.BaseDirectory,
	}

	volSize, err := p.unpackLayerToTemporaryDirectory(logger, unpackSpec, layerInfo, parentLayerInfo)
	if err != nil {
		return err
	}

	return p.finalizeVolume(logger, tempVolumeName, volumePath, layerInfo.ChainID, volSize)
}

func (p *BaseImagePuller) createTemporaryVolumeDirectory(logger lager.Logger, layerInfo groot.LayerInfo, spec groot.BaseImageSpec) (string, string, error) {
	tempVolumeName := fmt.Sprintf("%s-incomplete-%d-%d", layerInfo.ChainID, time.Now().UnixNano(), rand.Int())
	volumePath, err := p.volumeDriver.CreateVolume(logger,
		layerInfo.ParentChainID,
		tempVolumeName,
	)
	if err != nil {
		return "", "", errorspkg.Wrapf(err, "creating volume for layer `%s`", layerInfo.BlobID)
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

func (p *BaseImagePuller) unpackLayerToTemporaryDirectory(logger lager.Logger, unpackSpec UnpackSpec, layerInfo, parentLayerInfo groot.LayerInfo) (volSize int64, err error) {
	defer p.metricsEmitter.TryEmitDurationFrom(logger, MetricsUnpackTimeName, time.Now())

	if unpackSpec.BaseDirectory != "" {
		parentPath, err := p.volumeDriver.VolumePath(logger, parentLayerInfo.ChainID)
		if err != nil {
			return 0, err
		}

		if err := ensureBaseDirectoryExists(unpackSpec.BaseDirectory, unpackSpec.TargetPath, parentPath); err != nil {
			return 0, err
		}
	}

	var unpackOutput UnpackOutput
	if unpackOutput, err = p.unpacker.Unpack(logger, unpackSpec); err != nil {
		if errD := p.volumeDriver.DestroyVolume(logger, layerInfo.ChainID); errD != nil {
			logger.Error("volume-cleanup-failed", errD)
		}
		return 0, errorspkg.Wrapf(err, "unpacking layer `%s`", layerInfo.BlobID)
	}

	if err := p.volumeDriver.HandleOpaqueWhiteouts(logger, path.Base(unpackSpec.TargetPath), unpackOutput.OpaqueWhiteouts); err != nil {
		logger.Error("handling-opaque-whiteouts", err)
		return 0, errorspkg.Wrap(err, "handling opaque whiteouts")
	}

	logger.Debug("layer-unpacked")
	return unpackOutput.BytesWritten, nil
}

func (p *BaseImagePuller) finalizeVolume(logger lager.Logger, tempVolumeName, volumePath, chainID string, volSize int64) error {
	if err := p.volumeDriver.WriteVolumeMeta(logger, chainID, VolumeMeta{Size: volSize}); err != nil {
		return errorspkg.Wrapf(err, "writing volume `%s` metadata", chainID)
	}

	finalVolumePath := strings.Replace(volumePath, tempVolumeName, chainID, 1)
	if err := p.volumeDriver.MoveVolume(logger, volumePath, finalVolumePath); err != nil {
		return errorspkg.Wrapf(err, "failed to move volume to its final location")
	}

	return nil
}

func (p *BaseImagePuller) layersSize(layerInfos []groot.LayerInfo) int64 {
	var totalSize int64
	for _, layerInfo := range layerInfos {
		totalSize += layerInfo.Size
	}
	return totalSize
}

func ensureBaseDirectoryExists(baseDir, childPath, parentPath string) error {
	if baseDir == string(filepath.Separator) {
		return nil
	}

	if err := ensureBaseDirectoryExists(filepath.Dir(baseDir), childPath, parentPath); err != nil {
		return err
	}

	stat, err := os.Stat(filepath.Join(childPath, baseDir))
	if err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return errorspkg.Wrapf(err, "failed to stat base directory")
	}

	stat, err = os.Stat(filepath.Join(parentPath, baseDir))
	if err != nil {
		return errorspkg.Wrapf(err, "base directory not found in parent layer")
	}

	fullChildBaseDir := filepath.Join(childPath, baseDir)
	if err := os.Mkdir(fullChildBaseDir, stat.Mode()); err != nil {
		return errorspkg.Wrapf(err, "could not create base directory in child layer")
	}

	stat_t := stat.Sys().(*syscall.Stat_t)
	if err := os.Chown(fullChildBaseDir, int(stat_t.Uid), int(stat_t.Gid)); err != nil {
		return errorspkg.Wrapf(err, "could not chown base directory")
	}

	return nil
}
