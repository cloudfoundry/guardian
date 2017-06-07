package rundmc

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

//go:generate counterfeiter . BundlerRule
type BundlerRule interface {
	Apply(bndle goci.Bndl, spec gardener.DesiredContainerSpec, containerDir string) (goci.Bndl, error)
}

type BundleTemplate struct {
	Rules []BundlerRule
}

func (b BundleTemplate) Generate(spec gardener.DesiredContainerSpec, containerDir string) (goci.Bndl, error) {
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
