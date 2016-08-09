package cloner

import (
	"io"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type UnpackSpec struct {
	Stream      io.ReadCloser
	RootFSPath  string
	UIDMappings []groot.IDMappingSpec
	GIDMappings []groot.IDMappingSpec
}

//go:generate counterfeiter . Streamer
type Streamer interface {
	Stream(logger lager.Logger, source string) (io.ReadCloser, int64, error)
}

//go:generate counterfeiter . Unpacker
type Unpacker interface {
	Unpack(logger lager.Logger, spec UnpackSpec) error
}
