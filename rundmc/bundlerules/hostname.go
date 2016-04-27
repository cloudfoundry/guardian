package bundlerules

import (
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
)

type Hostname struct {
}

func (l Hostname) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	hostname := spec.Hostname
	if len(hostname) > 49 {
		hostname = hostname[len(hostname)-49:]
	}

	return bndl.WithHostname(hostname)
}
