package sandbox // import "code.cloudfoundry.org/grootfs/sandbox"

import (
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . IDMapper
type IDMapper interface {
	MapUIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error
	MapGIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error
}

type reexecer struct {
	logger     lager.Logger
	idMapper   IDMapper
	idMappings groot.IDMappings
}

func NewReexecer(logger lager.Logger, idMapper IDMapper, idMappings groot.IDMappings) groot.SandboxReexecer {
	return &reexecer{
		logger:     logger,
		idMapper:   idMapper,
		idMappings: idMappings,
	}
}
