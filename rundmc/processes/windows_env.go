package processes

import (
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
)

func WindowsEnvFor(bndl goci.Bndl, spec runrunc.ProcessSpec) []string {
	return append(bndl.Spec.Process.Env, spec.Env...)
}
