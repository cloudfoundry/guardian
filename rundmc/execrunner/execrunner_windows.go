package execrunner

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/lager"
)

type DirectExecRunner struct {
	RuntimePath   string
	CommandRunner commandrunner.CommandRunner
	RunMode       string
}

func (e *DirectExecRunner) Run(
	log lager.Logger, processID, processPath, sandboxHandle, _ string,
	_, _ uint32, pio garden.ProcessIO, _ bool, procJSON io.Reader,
	extraCleanup func() error,
) (garden.Process, error) {
	log = log.Session("execrunner")

	log.Info("start")
	defer log.Info("done")

	logR, logW, err := os.Pipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating log pipe")
	}
	defer logW.Close()

	var childLogW syscall.Handle

	// GetCurrentProcess doesn't error
	self, _ := syscall.GetCurrentProcess()
	// duplicate handle so it is inheritable by child process
	err = syscall.DuplicateHandle(self, syscall.Handle(logW.Fd()), self, &childLogW, 0, true, syscall.DUPLICATE_SAME_ACCESS)
	if err != nil {
		return nil, errors.Wrap(err, "duplicating log pipe handle")
	}

	cmd := exec.Command(e.RuntimePath, "--debug", "--log-handle", strconv.FormatUint(uint64(childLogW), 10), "--log-format", "json", e.RunMode, "--pid-file", filepath.Join(processPath, "pidfile"))

	if e.RunMode == "exec" {
		specPath := filepath.Join(processPath, "spec.json")
		if err := writeProcessJSON(procJSON, specPath); err != nil {
			return nil, err
		}
		cmd.Args = append(cmd.Args, "-p", specPath, sandboxHandle)
	} else {
		cmd.Args = append(cmd.Args, "--bundle", processPath, processID)
	}

	proc := &process{id: processID, cleanup: extraCleanup, logger: log}

	cmd.Stdout = pio.Stdout
	cmd.Stderr = pio.Stderr
	if err := e.CommandRunner.Start(cmd); err != nil {
		return nil, errors.Wrap(err, "execing runtime plugin")
	}

	proc.mux.Lock()
	go streamLogs(log, logR)

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
		// the streamLogs go func will block until this handle is closed
		syscall.CloseHandle(childLogW)
		proc.mux.Unlock()
	}()

	return proc, nil
}

func writeProcessJSON(procJSON io.Reader, specPath string) error {
	specFile, err := os.OpenFile(specPath, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return errors.Wrap(err, "opening process spec file for writing")
	}
	defer specFile.Close()
	if _, err := io.Copy(specFile, procJSON); err != nil {
		return errors.Wrap(err, "writing process spec")
	}

	return nil
}

func (e *DirectExecRunner) Attach(log lager.Logger, processID string, io garden.ProcessIO, processesPath string) (garden.Process, error) {
	panic("not supported on this platform")
}

type process struct {
	id       string
	exitCode int
	exitErr  error
	mux      sync.RWMutex
	cleanup  func() error
	logger   lager.Logger
}

func (p *process) ID() string {
	return p.id
}

func (p *process) Wait() (int, error) {
	p.mux.RLock()
	defer p.mux.RUnlock()

	if p.cleanup != nil {
		if err := p.cleanup(); err != nil {
			p.logger.Error("process-cleanup", err)
		}
	}

	return p.exitCode, p.exitErr
}

func (p *process) SetTTY(ttySpec garden.TTYSpec) error {
	return nil
}

func (p *process) Signal(signal garden.Signal) error {
	return nil
}

func streamLogs(logger lager.Logger, src *os.File) {
	defer src.Close()
	scanner := bufio.NewScanner(src)

	for scanner.Scan() {
		nextLogLine := scanner.Bytes()
		logging.ForwardRuncLogsToLager(logger, "winc", nextLogLine)
	}

	logger.Info("done-streaming-winc-logs")
}
