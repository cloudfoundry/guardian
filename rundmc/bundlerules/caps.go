package bundlerules

import (
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
)

type PrivilegedCaps struct {
}

func (r PrivilegedCaps) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	if !spec.Privileged {
		return bndl
	}

	bndl = bndl.WithCapabilities(
		append(bndl.Capabilities(), "CAP_SYS_ADMIN")...)

	return bndl
}
