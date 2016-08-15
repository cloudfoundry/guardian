package cloner

import (
	"fmt"
	"net/url"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type LayerDigest struct {
	LayerID string
	DiffID  string
	ChainID string
}

//go:generate counterfeiter . RemoteFetcher
type RemoteFetcher interface {
	LayersDigest(logger lager.Logger, imageURL *url.URL) ([]LayerDigest, error)
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

	digests, err := c.remoteFetcher.LayersDigest(logger, imageURL)
	if err != nil {
		return fmt.Errorf("fetching list of digests: %s", err)
	}
	logger.Debug("fetched-layers-digest", lager.Data{"digests": digests})

	streamer, err := c.remoteFetcher.Streamer(logger, imageURL)
	if err != nil {
		return fmt.Errorf("initializing streamer: %s", err)
	}
	for _, digest := range digests {
		stream, _, err := streamer.Stream(logger, digest.LayerID)
		if err != nil {
			return fmt.Errorf("streaming blob `%s`: %s", digest, err)
		}
		logger.Debug("got-stream-for-digest", lager.Data{"digest": digest})

		unpackSpec := UnpackSpec{
			Stream:      stream,
			RootFSPath:  spec.RootFSPath,
			UIDMappings: spec.UIDMappings,
			GIDMappings: spec.GIDMappings,
		}
		logger.Debug("blob-unpacking", lager.Data{
			"spec":   unpackSpec,
			"digest": digest,
		})
		if err := c.unpacker.Unpack(logger, unpackSpec); err != nil {
			return fmt.Errorf("unpacking blob `%s`: %s", digest, err)
		}
		logger.Debug("blob-unpacked", lager.Data{"digest": digest})
	}

	return nil
}
