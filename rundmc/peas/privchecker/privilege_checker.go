package privchecker

import (
	"fmt"

	"code.cloudfoundry.org/guardian/rundmc/runrunc"
)

type PrivilegeChecker struct {
	BundleLoader runrunc.BundleLoader
}

func (p *PrivilegeChecker) Privileged(bundlePath string) (bool, error) {

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
