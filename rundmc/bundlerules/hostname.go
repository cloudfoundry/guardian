package bundlerules

import (
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type Hostname struct {
}

func (r Hostname) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec) (goci.Bndl, error) {
	hostname := spec.Hostname
	if len(hostname) > 49 {
		hostname = hostname[len(hostname)-49:]
	}

	return bndl.WithHostname(hostname), nil
}
