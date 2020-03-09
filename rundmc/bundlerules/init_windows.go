package bundlerules

import (
	"path/filepath"

	gardenSpec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// Note - it's not possible to bind mount a single file in Windows, so we are
// using a directory instead
func (r Init) Apply(bndl goci.Bndl, spec gardenSpec.DesiredContainerSpec) (goci.Bndl, error) {
	initPathInContainer := filepath.Join(`C:\`, "Windows", "Temp", "bin", filepath.Base(r.InitPath))
	initMount := specs.Mount{
		Type:        "bind",
		Source:      filepath.Dir(r.InitPath),
		Destination: filepath.Dir(initPathInContainer),
		Options:     []string{"bind"},
	}
	process := bndl.Process()
	process.Args = []string{initPathInContainer}
	return bndl.WithMounts(initMount).WithProcess(process), nil
}
