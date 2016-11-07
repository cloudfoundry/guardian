package base_image_puller // import "code.cloudfoundry.org/grootfs/base_image_puller"

import (
	"fmt"
	"io"
	"net/url"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	errorspkg "github.com/pkg/errors"
)

//go:generate counterfeiter . VolumeDriver
//go:generate counterfeiter . Fetcher
//go:generate counterfeiter . Unpacker

type UnpackSpec struct {
	Stream      io.ReadCloser
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

type VolumeDriver interface {
	Path(logger lager.Logger, id string) (string, error)
	Create(logger lager.Logger, parentID, id string) (string, error)
	DestroyVolume(logger lager.Logger, id string) error
}

type Fetcher interface {
	BaseImageInfo(logger lager.Logger, baseImageURL *url.URL) (BaseImageInfo, error)
	StreamBlob(logger lager.Logger, baseImageURL *url.URL, source string) (io.ReadCloser, int64, error)
}

type Unpacker interface {
	Unpack(logger lager.Logger, spec UnpackSpec) error
}

type BaseImagePuller struct {
	localFetcher  Fetcher
	remoteFetcher Fetcher
	unpacker      Unpacker
	volumeDriver  VolumeDriver
}

func NewBaseImagePuller(localFetcher, remoteFetcher Fetcher, unpacker Unpacker, volumeDriver VolumeDriver) *BaseImagePuller {
	return &BaseImagePuller{
		localFetcher:  localFetcher,
		remoteFetcher: remoteFetcher,
		unpacker:      unpacker,
		volumeDriver:  volumeDriver,
	}
}

func (p *BaseImagePuller) Pull(logger lager.Logger, spec groot.BaseImageSpec) (groot.BaseImage, error) {
	logger = logger.Session("image-pulling", lager.Data{"spec": spec})
	logger.Info("start")
	defer logger.Info("end")
	var err error

	baseImageInfo, err := p.fetcher(spec.BaseImageSrc).BaseImageInfo(logger, spec.BaseImageSrc)
	if err != nil {
		return groot.BaseImage{}, errorspkg.Wrap(err, "fetching list of digests")
	}
	logger.Debug("fetched-layers-digests", lager.Data{"digests": baseImageInfo.LayersDigest})

	if err := p.quotaExceeded(logger, baseImageInfo.LayersDigest, spec); err != nil {
		return groot.BaseImage{}, err
	}

	volumePath, err := p.buildLayer(logger, len(baseImageInfo.LayersDigest)-1, baseImageInfo.LayersDigest, spec)
	if err != nil {
		return groot.BaseImage{}, err
	}
	chainIDs := p.chainIDs(baseImageInfo.LayersDigest)

	baseImage := groot.BaseImage{
		BaseImage:  baseImageInfo.Config,
		ChainIDs:   chainIDs,
		VolumePath: volumePath,
	}
	return baseImage, nil
}

func (p *BaseImagePuller) fetcher(baseImageURL *url.URL) Fetcher {
	if baseImageURL.Scheme == "" {
		return p.localFetcher
	} else {
		return p.remoteFetcher
	}
}

func (p *BaseImagePuller) quotaExceeded(logger lager.Logger, layersDigest []LayerDigest, spec groot.BaseImageSpec) error {
	if spec.ExcludeBaseImageFromQuota || spec.DiskLimit == 0 {
		return nil
	}

	totalSize := p.layersSize(layersDigest)
	if totalSize > spec.DiskLimit {
		err := fmt.Errorf("layers exceed disk quota %d/%d bytes", totalSize, spec.DiskLimit)
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

func (p *BaseImagePuller) buildLayer(logger lager.Logger, index int, layersDigest []LayerDigest, spec groot.BaseImageSpec) (string, error) {
	if index < 0 {
		return "", nil
	}

	digest := layersDigest[index]
	volumePath, err := p.volumeDriver.Path(logger, wrapVolumeID(spec, digest.ChainID))
	if err == nil {
		logger.Debug("volume-exists", lager.Data{
			"volumePath":    volumePath,
			"blobID":        digest.BlobID,
			"chainID":       digest.ChainID,
			"parentChainID": digest.ParentChainID,
		})
		return volumePath, nil
	}

	p.buildLayer(logger, index-1, layersDigest, spec)

	stream, size, err := p.fetcher(spec.BaseImageSrc).StreamBlob(logger, spec.BaseImageSrc, digest.BlobID)
	if err != nil {
		return "", fmt.Errorf("streaming blob `%s`: %s", digest.BlobID, err)
	}

	logger.Debug("got-stream-for-blob", lager.Data{
		"size":                      size,
		"diskLimit":                 spec.DiskLimit,
		"excludeBaseImageFromQuota": spec.ExcludeBaseImageFromQuota,
		"blobID":                    digest.BlobID,
		"chainID":                   digest.ChainID,
		"parentChainID":             digest.ParentChainID,
	})

	volumePath, err = p.volumeDriver.Create(logger,
		wrapVolumeID(spec, digest.ParentChainID),
		wrapVolumeID(spec, digest.ChainID),
	)
	if err != nil {
		return "", fmt.Errorf("creating volume for layer `%s`: %s", digest.BlobID, err)
	}
	logger.Debug("volume-created", lager.Data{
		"volumePath":    volumePath,
		"blobID":        digest.BlobID,
		"chainID":       digest.ChainID,
		"parentChainID": digest.ParentChainID,
	})

	unpackSpec := UnpackSpec{
		TargetPath:  volumePath,
		Stream:      stream,
		UIDMappings: spec.UIDMappings,
		GIDMappings: spec.GIDMappings,
	}

	if err := p.unpacker.Unpack(logger, unpackSpec); err != nil {
		if errD := p.volumeDriver.DestroyVolume(logger, wrapVolumeID(spec, digest.ChainID)); errD != nil {
			logger.Error("volume-cleanup-failed", errD, lager.Data{
				"blobID":        digest.BlobID,
				"chainID":       digest.ChainID,
				"parentChainID": digest.ParentChainID,
			})
		}
		return "", fmt.Errorf("unpacking layer `%s`: %s", digest.BlobID, err)
	}
	logger.Debug("layer-unpacked", lager.Data{
		"blobID":        digest.BlobID,
		"chainID":       digest.ChainID,
		"parentChainID": digest.ParentChainID,
	})

	return volumePath, nil
}

func (p *BaseImagePuller) layersSize(layerDigests []LayerDigest) int64 {
	var totalSize int64
	for _, digest := range layerDigests {
		totalSize += digest.Size
	}
	return totalSize
}

func wrapVolumeID(spec groot.BaseImageSpec, volumeID string) string {
	if volumeID == "" {
		return ""
	}

	if len(spec.UIDMappings) > 0 || len(spec.GIDMappings) > 0 {
		return fmt.Sprintf("%s-namespaced", volumeID)
	}

	return volumeID
}
