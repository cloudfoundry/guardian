package rundmc

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"

	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/pivotal-golang/lager"
)

type nstar struct {
	NstarBinPath string
	TarBinPath   string

	CommandRunner command_runner.CommandRunner
}

func NewNstarRunner(nstarPath, tarPath string, runner command_runner.CommandRunner) NstarRunner {
	return &nstar{
		NstarBinPath:  nstarPath,
		TarBinPath:    tarPath,
		CommandRunner: runner,
	}
}

func (n *nstar) StreamIn(logger lager.Logger, pid int, path string, user string, tarStream io.Reader) error {
	buff := new(bytes.Buffer)
	cmd := exec.Command(n.NstarBinPath, n.TarBinPath, fmt.Sprintf("%d", pid), user, path)
	cmd.Stdout = buff
	cmd.Stderr = buff
	cmd.Stdin = tarStream

	if err := n.CommandRunner.Run(cmd); err != nil {
		return fmt.Errorf("error streaming in: %v. Output: %s", err, buff.String())
	}

	return nil
}
