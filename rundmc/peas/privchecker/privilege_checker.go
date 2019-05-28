package privchecker

import (
	"fmt"

	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Depot
type Depot interface {
	Lookup(log lager.Logger, handle string) (path string, err error)
}

type PrivilegeChecker struct {
	BundleLoader runrunc.BundleLoader
	Depot        Depot
	Log          lager.Logger
}

func (p *PrivilegeChecker) Privileged(id string) (bool, error) {
	bundlePath, err := p.Depot.Lookup(p.Log, id)
	if err != nil {
		return false, fmt.Errorf("looking up bundle: %s", err)
	}

	bundle, err := p.BundleLoader.Load(bundlePath)
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
