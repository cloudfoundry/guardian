package cloner

import (
	"fmt"
	"net/url"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . RemoteFetcher
type RemoteFetcher interface {
	LayersDigest(logger lager.Logger, imageURL *url.URL) ([]string, error)
	Streamer(logger lager.Logger, imageURL *url.URL) (Streamer, error)
}

type RemoteCloner struct {
	remoteFetcher RemoteFetcher
	unpacker      Unpacker
}

func NewRemoteCloner(remoteFetcher RemoteFetcher, unpacker Unpacker) *RemoteCloner {
	return &RemoteCloner{
		remoteFetcher: remoteFetcher,
		unpacker:      unpacker,
	}
}

func (c *RemoteCloner) Clone(logger lager.Logger, spec groot.CloneSpec) error {
	logger = logger.Session("remote-cloning", lager.Data{"spec": spec})
	logger.Debug("start")
	defer logger.Debug("end")

	imageURL, err := url.Parse(spec.Image)
	if err != nil {
		return fmt.Errorf("parsing URL: %s", err)
	}

	layers, err := c.remoteFetcher.LayersDigest(logger, imageURL)
	if err != nil {
		return fmt.Errorf("fetching list of layers: %s", err)
	}
	logger.Debug("fetched-layers-digest", lager.Data{"layers": layers})

	streamer, err := c.remoteFetcher.Streamer(logger, imageURL)
	if err != nil {
		return fmt.Errorf("initializing streamer: %s", err)
	}
	for _, layer := range layers {
		stream, _, err := streamer.Stream(logger, layer)
		if err != nil {
			return fmt.Errorf("streaming blob `%s`: %s", layer, err)
		}
		logger.Debug("got-stream-for-layer", lager.Data{"layer": layer})

		unpackSpec := UnpackSpec{
			Stream:      stream,
			RootFSPath:  spec.RootFSPath,
			UIDMappings: spec.UIDMappings,
			GIDMappings: spec.GIDMappings,
		}
		logger.Debug("layer-unpacking", lager.Data{
			"spec":  unpackSpec,
			"layer": layer,
		})
		if err := c.unpacker.Unpack(logger, unpackSpec); err != nil {
			return fmt.Errorf("unpacking blob `%s`: %s", layer, err)
		}
		logger.Debug("layer-unpacked", lager.Data{"layer": layer})
	}

	return nil
}
