package bundlerules

import (
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
)

type Base struct {
	PrivilegedBase   goci.Bndl
	UnprivilegedBase goci.Bndl
}

func (r Base) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec) goci.Bndl {
	if spec.Privileged {
		return r.PrivilegedBase
	} else {
		return r.UnprivilegedBase
	}
}
