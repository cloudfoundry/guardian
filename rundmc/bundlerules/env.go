package bundlerules

import (
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type Env struct {
}

func (r Env) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec) (goci.Bndl, error) {
	process := bndl.Process()
	var baseEnv []string
	if spec.BaseConfig.Process != nil {
		baseEnv = spec.BaseConfig.Process.Env
	}
	process.Env = append(baseEnv, spec.Env...)
	return bndl.WithProcess(process), nil
}
