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

	hooks := bndl.WithPrestartHooks(specs.Hook{
		Env:  env,
		Path: spec.NetworkHooks.Prestart.Path,
		Args: spec.NetworkHooks.Prestart.Args,
	})

	if spec.NetworkHooks.Poststop.Path != "" {
		hooks = hooks.WithPoststopHooks(specs.Hook{
			Env:  env,
			Path: spec.NetworkHooks.Poststop.Path,
			Args: spec.NetworkHooks.Poststop.Args,
		})
	}

	return hooks
}
