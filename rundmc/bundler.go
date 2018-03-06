package rundmc

import (
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

//go:generate counterfeiter . BundlerRule
type BundlerRule interface {
	Apply(bndle goci.Bndl, desiredContainerSpec spec.DesiredContainerSpec, containerDir string) (goci.Bndl, error)
}

type BundleTemplate struct {
	Rules []BundlerRule
}

func (b BundleTemplate) Generate(spec spec.DesiredContainerSpec, containerDir string) (goci.Bndl, error) {
	var bndl goci.Bndl

	for _, rule := range b.Rules {
		var err error
		bndl, err = rule.Apply(bndl, spec, containerDir)
		if err != nil {
			return goci.Bndl{}, err
		}
	}

	return bndl, nil
}
