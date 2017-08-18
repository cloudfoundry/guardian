package unpacker // import "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"

import (
	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/lager"
)

type UnpackStrategy struct {
	Name               string
	WhiteoutDevicePath string
}

type TarUnpacker struct{}

func NewTarUnpacker(unpackStrategy UnpackStrategy) (*TarUnpacker, error) {
	panic("not implemented")
}

func (u *TarUnpacker) Unpack(logger lager.Logger, spec base_image_puller.UnpackSpec) error {
	panic("not implemented")
}
