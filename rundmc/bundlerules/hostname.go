package bundlerules

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type Hostname struct {
}

func (l Hostname) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec, _ string) (goci.Bndl, error) {
	hostname := spec.Hostname
	if len(hostname) > 49 {
		hostname = hostname[len(hostname)-49:]
	}

	return bndl.WithHostname(hostname), nil
}
