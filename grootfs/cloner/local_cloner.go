package cloner

import (
	"fmt"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type LocalCloner struct {
	unpacker Unpacker
	streamer Streamer
}

func NewLocalCloner(streamer Streamer, unpacker Unpacker) *LocalCloner {
	return &LocalCloner{
		streamer: streamer,
		unpacker: unpacker,
	}
}

func (c *LocalCloner) Clone(logger lager.Logger, spec groot.CloneSpec) error {
	logger = logger.Session("local-cloning", lager.Data{"spec": spec})
	logger.Debug("start")
	defer logger.Debug("end")

	stream, _, err := c.streamer.Stream(logger, spec.Image)
	if err != nil {
		return fmt.Errorf("reading from `%s`: %s", spec.Image, err)
	}

	if err := c.unpacker.Unpack(logger, UnpackSpec{
		Stream:      stream,
		TargetPath:  spec.RootFSPath,
		UIDMappings: spec.UIDMappings,
		GIDMappings: spec.GIDMappings,
	}); err != nil {
		return fmt.Errorf("writing to `%s`: %s", spec.RootFSPath, err)
	}

	return nil
}
