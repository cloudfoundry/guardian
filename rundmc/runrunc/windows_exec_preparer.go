package runrunc

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// The fact that this file is named windows_exec_preparer.go rather than
// exec_preparer_windows.go is intentional: the WindowsExecPreparer doesn't
// actually need to be developed only on Windows.
type WindowsExecPreparer struct{}

func (*WindowsExecPreparer) Prepare(log lager.Logger, bundlePath string, spec garden.ProcessSpec) (*PreparedSpec, error) {
	return &PreparedSpec{
		Process: specs.Process{
			Args: append([]string{spec.Path}, spec.Args...),
		},
	}, nil
}
