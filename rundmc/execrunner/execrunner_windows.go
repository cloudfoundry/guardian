package execrunner

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

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
}

func (e *DirectExecRunner) Run(log lager.Logger, processID string, spec *runrunc.PreparedSpec, bundlePath, processesPath, handle string, tty *garden.TTYSpec, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("execrunner")

	log.Info("start")
	defer log.Info("done")

	if processID == "" {
		processID = e.ProcessIDGen.Generate()
	}

	proc := &process{id: processID}
	marshaledSpec, err := json.Marshal(spec)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling process spec")
	}

	processPath := filepath.Join(processesPath, proc.ID())
	_, err = os.Stat(processPath)
	if err == nil {
		return nil, errors.New(fmt.Sprintf("process ID '%s' already in use", processID))
	}

	if err := os.MkdirAll(processPath, 0700); err != nil {
		return nil, err
	}

	specPath := filepath.Join(processPath, "spec.json")
	if err := ioutil.WriteFile(specPath, marshaledSpec, 0600); err != nil {
		return nil, errors.Wrap(err, "writing process spec")
	}

	cmd := exec.Command(e.RuntimePath, "--debug", "--log", "IGNORED", "exec", "-p", specPath, "--pid-file", filepath.Join(processPath, "pidfile"), handle)
	cmd.Stdout = io.Stdout
	cmd.Stderr = io.Stderr
	if err := e.CommandRunner.Start(cmd); err != nil {
		return nil, errors.Wrap(err, "execing runtime plugin")
	}

	proc.mux.Lock()

	go func() {
		if err := e.CommandRunner.Wait(cmd); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					proc.exitCode = status.ExitStatus()
				} else {
					proc.exitCode = 1
					proc.exitErr = errors.New("couldn't get WaitStatus")
				}
			} else {
				proc.exitCode = 1
				proc.exitErr = err
			}
		}
		proc.mux.Unlock()
	}()

	return proc, nil
}

func (e *DirectExecRunner) Attach(log lager.Logger, processID string, io garden.ProcessIO, processesPath string) (garden.Process, error) {
	panic("not supported on this platform")
}

type process struct {
	id       string
	exitCode int
	exitErr  error
	mux      sync.RWMutex
}

func (p *process) ID() string {
	return p.id
}

func (p *process) Wait() (int, error) {
	p.mux.RLock()
	defer p.mux.RUnlock()

	return p.exitCode, p.exitErr
}

func (p *process) SetTTY(ttySpec garden.TTYSpec) error {
	return nil
}

func (p *process) Signal(signal garden.Signal) error {
	return nil
}
