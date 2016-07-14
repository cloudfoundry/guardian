package bundlerules

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type Hooks struct {
	LogFilePattern string
}

func (r Hooks) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec) goci.Bndl {
	env := []string{fmt.Sprintf(
		"GARDEN_LOG_FILE="+r.LogFilePattern, spec.Handle),
		"PATH=" + os.Getenv("PATH"),
	}

	var prestart, poststop []specs.Hook

	for _, networkHooks := range spec.NetworkHooks {
		if networkHooks.Prestart.Path != "" {
			prestart = append(prestart, specs.Hook{
				Env:  env,
				Path: networkHooks.Prestart.Path,
				Args: networkHooks.Prestart.Args,
			})
		}

		if networkHooks.Poststop.Path != "" {
			poststop = append(poststop, specs.Hook{
				Env:  env,
				Path: networkHooks.Poststop.Path,
				Args: networkHooks.Poststop.Args,
			})
		}
	}

	return bndl.WithPrestartHooks(prestart...).WithPoststopHooks(reverse(poststop)...)
}

func reverse(hooks []specs.Hook) []specs.Hook {
	for i, j := 0, len(hooks)-1; i < j; i, j = i+1, j-1 {
		hooks[i], hooks[j] = hooks[j], hooks[i]
	}

	return hooks
}
