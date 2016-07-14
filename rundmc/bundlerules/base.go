package bundlerules

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/mitchellh/copystructure"
)

type Base struct {
	PrivilegedBase   goci.Bndl
	UnprivilegedBase goci.Bndl
}

func (r Base) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec) goci.Bndl {
	if spec.Privileged {
		copiedBndl, err := copystructure.Copy(r.PrivilegedBase)
		if err != nil {
			panic(err)
		}
		return copiedBndl.(goci.Bndl)
	} else {
		copiedBndl, err := copystructure.Copy(r.UnprivilegedBase)
		if err != nil {
			panic(err)
		}
		return copiedBndl.(goci.Bndl)
	}
}
