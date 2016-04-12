package bundlerules

import (
	"fmt"
	"os"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/opencontainers/specs/specs-go"
)

type Hooks struct {
	LogFilePattern string
}

func (r Hooks) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
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

	for i, j := 0, len(poststop)-1; i < j; i, j = i+1, j-1 {
		poststop[i], poststop[j] = poststop[j], poststop[i]
	}

	return bndl.WithPrestartHooks(prestart...).WithPoststopHooks(poststop...)
}
