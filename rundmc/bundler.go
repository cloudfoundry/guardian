package rundmc

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

//go:generate counterfeiter . BundlerRule
type BundlerRule interface {
	Apply(bndle goci.Bndl, spec gardener.DesiredContainerSpec) goci.Bndl
}

type BundleTemplate struct {
	Rules []BundlerRule
}

func (b BundleTemplate) Generate(spec gardener.DesiredContainerSpec) goci.Bndl {
	var bndl goci.Bndl

	for _, rule := range b.Rules {
		bndl = rule.Apply(bndl, spec)
	}

	return bndl
}
