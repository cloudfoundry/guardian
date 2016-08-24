package cloner

import (
	"io"
	"net/url"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
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
	LayersDigest(logger lager.Logger, imageURL *url.URL) ([]LayerDigest, specsv1.Image, error)
	Streamer(logger lager.Logger, imageURL *url.URL) (Streamer, error)
}

//go:generate counterfeiter . Streamer
type Streamer interface {
	Stream(logger lager.Logger, source string) (io.ReadCloser, int64, error)
}

//go:generate counterfeiter . Unpacker
type Unpacker interface {
	Unpack(logger lager.Logger, spec UnpackSpec) error
}
