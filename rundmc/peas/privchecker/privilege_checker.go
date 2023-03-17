package privchecker

import (
	"fmt"

	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager/v3"
)

//go:generate counterfeiter . BundleLoader
type BundleLoader interface {
	Load(log lager.Logger, handle string) (bundle goci.Bndl, err error)
}

type PrivilegeChecker struct {
	BundleLoader BundleLoader
	Log          lager.Logger
}

func (p *PrivilegeChecker) Privileged(id string) (bool, error) {
	bundle, err := p.BundleLoader.Load(p.Log, id)
	if err != nil {
		return false, fmt.Errorf("loading bundle: %s", err)
	}

	for _, namespace := range bundle.Spec.Linux.Namespaces {
		if namespace.Type == "user" {
			return false, nil
		}
	}

	return true, nil
}
