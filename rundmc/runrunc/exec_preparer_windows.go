package runrunc

import (
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type WindowsExecPreparer struct{}

func (*WindowsExecPreparer) Prepare(log lager.Logger, bundlePath string, spec garden.ProcessSpec) (*PreparedSpec, error) {
	cwd := "C:\\"

	if spec.Dir != "" {
		cwd = spec.Dir
	}

	volumeName := filepath.VolumeName(cwd)

	if err := os.MkdirAll(filepath.Join(bundlePath, "mnt", strings.Replace(cwd, volumeName, "", 1)), 0755); err != nil {
		return nil, err
	}

	return &PreparedSpec{
		Process: specs.Process{
			Args: append([]string{spec.Path}, spec.Args...),
			Cwd:  cwd,
		},
	}, nil
}
