package runrunc

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

func WindowsEnvFor(uid int, bndl goci.Bndl, spec garden.ProcessSpec) []string {
	return append(bndl.Spec.Process.Env, spec.Env...)
}
