package bundlerules

import (
	gardenSpec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	ociSpec "github.com/opencontainers/runtime-spec/specs-go"
)

func (r Init) Apply(bndl goci.Bndl, spec gardenSpec.DesiredContainerSpec) (goci.Bndl, error) {
	destination := "/tmp/garden-init"
	if spec.SandboxHandle != "" {
		destination = "/tmp/garden-pea-init"
	}

	process := bndl.Process()
	process.Args = []string{destination}
	initMount := ociSpec.Mount{
		Destination: destination,
		Source:      r.InitPath,
		Type:        "bind",
		Options:     []string{"bind"},
	}
	return bndl.WithMounts(initMount).WithProcess(process), nil
}
