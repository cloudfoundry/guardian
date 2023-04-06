package runcontainerd

import (
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager/v3"
)

type BndlLoader struct {
	containerManager ContainerManager
}

func NewBndlLoader(containerManager ContainerManager) BndlLoader {
	return BndlLoader{
		containerManager: containerManager,
	}
}

func (b BndlLoader) Load(log lager.Logger, handle string) (goci.Bndl, error) {
	spec, err := b.containerManager.Spec(log, handle)
	if err != nil {
		return goci.Bndl{}, err
	}

	return goci.Bndl{Spec: *spec}, nil

}
