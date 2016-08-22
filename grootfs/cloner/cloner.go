package cloner

import (
	"io"
	"net/url"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type UnpackSpec struct {
	Stream      io.ReadCloser
	TargetPath  string
	UIDMappings []groot.IDMappingSpec
	GIDMappings []groot.IDMappingSpec
}

type LayerDigest struct {
	BlobID        string
	DiffID        string
	ChainID       string
	ParentChainID string
}

//go:generate counterfeiter . Fetcher
type Fetcher interface {
	LayersDigest(logger lager.Logger, imageURL *url.URL) ([]LayerDigest, error)
	Streamer(logger lager.Logger, imageURL *url.URL) (Streamer, error)
}

//go:generate counterfeiter . VolumeDriver
type VolumeDriver interface {
	Path(logger lager.Logger, id string) (string, error)
	Create(logger lager.Logger, parentID, id string) (string, error)
	Snapshot(logger lager.Logger, id, path string) error
	Destroy(logger lager.Logger, path string) error
}

//go:generate counterfeiter . Streamer
type Streamer interface {
	Stream(logger lager.Logger, source string) (io.ReadCloser, int64, error)
}

//go:generate counterfeiter . Unpacker
type Unpacker interface {
	Unpack(logger lager.Logger, spec UnpackSpec) error
}
