package bundlerules

import (
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/goci/specs"
	"github.com/cloudfoundry-incubator/guardian/gardener"
)

type InitProcess struct {
	Process specs.Process
}

func (r InitProcess) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	r.Process.Env = append(r.Process.Env, spec.Env...)

	return bndl.WithProcess(r.Process)
}
