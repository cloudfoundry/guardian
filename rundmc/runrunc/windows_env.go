package runrunc

import (
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

func WindowsEnvFor(bndl goci.Bndl, spec ProcessSpec) []string {
	return append(bndl.Spec.Process.Env, spec.Env...)
}
