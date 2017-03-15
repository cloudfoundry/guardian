package unpacker // import "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"

import (
	"errors"

	"code.cloudfoundry.org/commandrunner"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/lager"
)

func NewNSIdMapperUnpacker(commandRunner commandrunner.CommandRunner, idMapper IDMapper, strategy UnpackStrategy) *NSIdMapperUnpacker {
	return &NSIdMapperUnpacker{
		commandRunner:  commandRunner,
		idMapper:       idMapper,
		unpackStrategy: strategy,
	}
}

func (u *NSIdMapperUnpacker) Unpack(logger lager.Logger, spec base_image_puller.UnpackSpec) error {
	return errors.New("Imcompatible with windows")
}
