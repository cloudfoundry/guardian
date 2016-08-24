package cloner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type LocalCloner struct {
	unpacker     Unpacker
	streamer     Streamer
	volumeDriver groot.VolumeDriver
}

func NewLocalCloner(streamer Streamer, unpacker Unpacker, volumizer groot.VolumeDriver) *LocalCloner {
	return &LocalCloner{
		streamer:     streamer,
		unpacker:     unpacker,
		volumeDriver: volumizer,
	}
}

func (c *LocalCloner) Clone(logger lager.Logger, spec groot.CloneSpec) error {
	logger = logger.Session("local-cloning", lager.Data{"spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	stat, err := os.Stat(spec.Image)
	if err != nil {
		return fmt.Errorf("checking image source: %s", err)
	}

	volumeID := c.generateVolumeID(spec.Image, int64(stat.ModTime().Nanosecond()))
	logger.Debug("checking-volumeid", lager.Data{"volumeID": volumeID})

	if _, err := c.volumeDriver.Path(logger, volumeID); err != nil {
		logger.Debug("creating-volume", lager.Data{"volumeID": volumeID})
		volumePath, err := c.volumeDriver.Create(logger, "", volumeID)
		if err != nil {
			return fmt.Errorf("creating volume `%s`: %s", volumeID, err)
		}

		logger.Debug("creating-stream", lager.Data{"volumeID": volumeID})
		stream, _, err := c.streamer.Stream(logger, spec.Image)
		if err != nil {
			return fmt.Errorf("reading from `%s`: %s", spec.Image, err)
		}

		logger.Debug("unpacking", lager.Data{"volumeID": volumeID})
		if err := c.unpacker.Unpack(logger, UnpackSpec{
			Stream:      stream,
			TargetPath:  volumePath,
			UIDMappings: spec.UIDMappings,
			GIDMappings: spec.GIDMappings,
		}); err != nil {
			return fmt.Errorf("writing to `%s`: %s", spec.Bundle.RootFSPath(), err)
		}
	}

	logger.Debug("snapshotting-to-rootfs", lager.Data{"volumeID": volumeID, "rootfsPath": spec.Bundle.RootFSPath()})
	if err := c.volumeDriver.Snapshot(logger, volumeID, spec.Bundle.RootFSPath()); err != nil {
		return fmt.Errorf("snapshotting the image `%s` to path `%s`: %s", volumeID, spec.Bundle.RootFSPath(), err)
	}

	logger.Debug("volume-snapshotted-to-rootfs", lager.Data{"volumeID": volumeID, "rootfsPath": spec.Bundle.RootFSPath()})
	return nil
}

func (c *LocalCloner) generateVolumeID(imagePath string, timestamp int64) string {
	imagePathSha := sha256.Sum256([]byte(imagePath))
	return fmt.Sprintf("%s-%d", hex.EncodeToString(imagePathSha[:32]), timestamp)
}
