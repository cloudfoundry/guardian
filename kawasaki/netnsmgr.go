package kawasaki

import (
	"os"
	"os/exec"
	"path"

	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/pivotal-golang/lager"
)

type mgr struct {
	runner   command_runner.CommandRunner
	netnsDir string
}

func NewManager(runner command_runner.CommandRunner, netnsDir string) NetnsMgr {
	return &mgr{
		runner:   runner,
		netnsDir: netnsDir,
	}
}

// Create creates a namespace using 'ip netns add' and
// runs a configurer against it to set it up.
func (m *mgr) Create(log lager.Logger, handle string) error {
	return m.runner.Run(exec.Command("ip", "netns", "add", handle))
}

func (m *mgr) Lookup(log lager.Logger, handle string) (string, error) {
	nspath := path.Join(m.netnsDir, handle)
	if _, err := os.Stat(nspath); os.IsNotExist(err) {
		return "", err
	}

	return nspath, nil
}

func (m *mgr) Destroy(log lager.Logger, handle string) error {
	return m.runner.Run(exec.Command("ip", "netns", "delete", handle))
}
