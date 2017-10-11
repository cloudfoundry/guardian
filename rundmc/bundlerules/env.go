package bundlerules

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type Env struct {
}

func (r Env) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec, _ string) (goci.Bndl, error) {
	process := bndl.Process()
	var baseEnv []string
	if spec.BaseConfig.Process != nil {
		baseEnv = spec.BaseConfig.Process.Env
	}
	process.Env = append(baseEnv, spec.Env...)
	return bndl.WithProcess(process), nil
}
