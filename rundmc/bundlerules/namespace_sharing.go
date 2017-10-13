package bundlerules

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type NamespaceSharing struct{}

func (n NamespaceSharing) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec, containerDir string) (goci.Bndl, error) {
	originalCtrInitPid, err := ioutil.ReadFile(filepath.Join(containerDir, "pidfile"))
	if err != nil {
		return goci.Bndl{}, err
	}

	return bndl.WithNamespaces(
		specs.LinuxNamespace{Type: "mount"},
		specs.LinuxNamespace{Type: "network", Path: fmt.Sprintf("/proc/%s/ns/net", string(originalCtrInitPid))},
		specs.LinuxNamespace{Type: "user", Path: fmt.Sprintf("/proc/%s/ns/user", string(originalCtrInitPid))},
		specs.LinuxNamespace{Type: "ipc", Path: fmt.Sprintf("/proc/%s/ns/ipc", string(originalCtrInitPid))},
		specs.LinuxNamespace{Type: "pid", Path: fmt.Sprintf("/proc/%s/ns/pid", string(originalCtrInitPid))},
		specs.LinuxNamespace{Type: "uts", Path: fmt.Sprintf("/proc/%s/ns/uts", string(originalCtrInitPid))},
	), nil
}
