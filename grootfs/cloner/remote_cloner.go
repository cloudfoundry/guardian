package cloner

import (
	"fmt"
	"net/url"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type RemoteCloner struct {
	fetcher      Fetcher
	unpacker     Unpacker
	volumeDriver groot.VolumeDriver
}

func NewRemoteCloner(fetcher Fetcher, unpacker Unpacker, volumizer groot.VolumeDriver) *RemoteCloner {
	return &RemoteCloner{
		fetcher:      fetcher,
		unpacker:     unpacker,
		volumeDriver: volumizer,
	}
}

func (c *RemoteCloner) Clone(logger lager.Logger, spec groot.CloneSpec) error {
	logger = logger.Session("remote-cloning", lager.Data{"spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	imageURL, err := url.Parse(spec.Image)
	if err != nil {
		return fmt.Errorf("parsing URL: %s", err)
	}

	digests, err := c.fetcher.LayersDigest(logger, imageURL)
	if err != nil {
		return fmt.Errorf("fetching list of digests: %s", err)
	}
	logger.Debug("fetched-layers-digests", lager.Data{"digests": digests})

	streamer, err := c.fetcher.Streamer(logger, imageURL)
	if err != nil {
		return fmt.Errorf("initializing streamer: %s", err)
	}

	for _, digest := range digests {
		volumePath, err := c.volumeDriver.Path(logger, wrapVolumeID(spec, digest.ChainID))
		if err == nil {
			logger.Debug("volume-exists", lager.Data{
				"volumePath":    volumePath,
				"blobID":        digest.BlobID,
				"diffID":        digest.DiffID,
				"chainID":       digest.ChainID,
				"parentChainID": digest.ParentChainID,
			})
			continue
		}

		volumePath, err = c.volumeDriver.Create(logger,
			wrapVolumeID(spec, digest.ParentChainID),
			wrapVolumeID(spec, digest.ChainID),
		)
		if err != nil {
			return fmt.Errorf("creating volume for layer `%s`: %s", digest.DiffID, err)
		}
		logger.Debug("volume-created", lager.Data{
			"volumePath":    volumePath,
			"blobID":        digest.BlobID,
			"diffID":        digest.DiffID,
			"chainID":       digest.ChainID,
			"parentChainID": digest.ParentChainID,
		})

		stream, size, err := streamer.Stream(logger, digest.BlobID)
		if err != nil {
			return fmt.Errorf("streaming blob `%s`: %s", digest.BlobID, err)
		}
		logger.Debug("got-stream-for-blob", lager.Data{
			"size":          size,
			"blobID":        digest.BlobID,
			"diffID":        digest.DiffID,
			"chainID":       digest.ChainID,
			"parentChainID": digest.ParentChainID,
		})

		unpackSpec := UnpackSpec{
			TargetPath:  volumePath,
			Stream:      stream,
			UIDMappings: spec.UIDMappings,
			GIDMappings: spec.GIDMappings,
		}
		if err := c.unpacker.Unpack(logger, unpackSpec); err != nil {
			return fmt.Errorf("unpacking layer `%s`: %s", digest.DiffID, err)
		}
		logger.Debug("layer-unpacked", lager.Data{
			"blobID":        digest.BlobID,
			"diffID":        digest.DiffID,
			"chainID":       digest.ChainID,
			"parentChainID": digest.ParentChainID,
		})
	}

	lastVolumeID := wrapVolumeID(spec, digests[len(digests)-1].ChainID)
	if err := c.volumeDriver.Snapshot(logger, lastVolumeID, spec.RootFSPath); err != nil {
		return fmt.Errorf("snapshotting the image to path `%s`: %s", spec.RootFSPath, err)
	}
	logger.Debug("last-volume-got-snapshotted", lager.Data{
		"lastVolumeID": lastVolumeID,
		"rootFSPath":   spec.RootFSPath,
	})

	return nil
}

func wrapVolumeID(spec groot.CloneSpec, volumeID string) string {
	if volumeID == "" {
		return ""
	}

	if len(spec.UIDMappings) > 0 || len(spec.GIDMappings) > 0 {
		return fmt.Sprintf("%s-namespaced", volumeID)
	}

	return volumeID
}
