package execrunner

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
)

type DirectExecRunner struct {
	RuntimePath   string
	CommandRunner commandrunner.CommandRunner
	ProcessIDGen  runrunc.UidGenerator
	FileWriter    FileWriter
}

//go:generate counterfeiter . FileWriter
type FileWriter interface {
	WriteFile(path string, contents []byte, mode os.FileMode) error
}

func (e *DirectExecRunner) Run(log lager.Logger, processID string, spec *runrunc.PreparedSpec, bundlePath, processesPath, handle string, tty *garden.TTYSpec, io garden.ProcessIO) (garden.Process, error) {
	if processID == "" {
		processID = e.ProcessIDGen.Generate()
	}

	proc := &process{id: processID}
	marshaledSpec, err := json.Marshal(spec)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling process spec")
	}

	specPath := filepath.Join(processesPath, proc.ID(), "spec.json")
	if err := e.FileWriter.WriteFile(specPath, marshaledSpec, 0600); err != nil {
		return nil, errors.Wrap(err, "writing process spec")
	}

	cmd := exec.Command(e.RuntimePath, "--debug", "--log", "IGNORED", "exec", "-d", "-p", specPath, "--pid-file", "IGNORED", handle)
	if err := e.CommandRunner.Start(cmd); err != nil {
		return nil, errors.Wrap(err, "execing runtime plugin")
	}

	return proc, nil
}

func (e *DirectExecRunner) Attach(log lager.Logger, processID string, io garden.ProcessIO, processesPath string) (garden.Process, error) {
	panic("not supported on this platform")
}

type process struct {
	id string
}

func (p *process) ID() string {
	return p.id
}

func (p *process) Wait() (int, error) {
	return 0, nil
}

func (p *process) SetTTY(ttySpec garden.TTYSpec) error {
	return nil
}

func (p *process) Signal(signal garden.Signal) error {
	return nil
}
