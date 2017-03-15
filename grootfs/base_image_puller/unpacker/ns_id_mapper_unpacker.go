package unpacker // import "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"

import (
	"code.cloudfoundry.org/commandrunner"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . IDMapper
type IDMapper interface {
	MapUIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error
	MapGIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error
}

type NSIdMapperUnpacker struct {
	commandRunner  commandrunner.CommandRunner
	idMapper       IDMapper
	unpackStrategy UnpackStrategy
}
