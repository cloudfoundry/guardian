package execrunner

import (
	"bufio"
	"fmt"
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
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
)

type WindowsExecRunner struct {
	runtimePath     string
	commandRunner   commandrunner.CommandRunner
	processes       map[string]*process
	processMux      *sync.Mutex
	bundleSaver     depot.BundleSaver
	bundleLookupper depot.BundleLookupper
	processDepot    ProcessDepot
}

func NewWindowsExecRunner(runtimePath string, commandRunner commandrunner.CommandRunner, bundleSaver depot.BundleSaver, bundleLookupper depot.BundleLookupper, processDepot ProcessDepot) *WindowsExecRunner {
	return &WindowsExecRunner{
		runtimePath:     runtimePath,
		commandRunner:   commandRunner,
		processes:       map[string]*process{},
		processMux:      new(sync.Mutex),
		bundleSaver:     bundleSaver,
		bundleLookupper: bundleLookupper,
		processDepot:    processDepot,
	}
}

type process struct {
	id           string
	exitCode     int
	exitErr      error
	exitMutex    *sync.RWMutex
	cleanup      func() error
	logger       lager.Logger
	stdoutWriter *DynamicMultiWriter
	stderrWriter *DynamicMultiWriter
	stdin        *os.File
	stdout       *os.File
	stderr       *os.File
	outputWg     *sync.WaitGroup
}

func (e *WindowsExecRunner) Run(
	log lager.Logger, processID, sandboxHandle string,
	pio garden.ProcessIO, _ bool, procJSON io.Reader, extraCleanup func() error,
) (garden.Process, error) {
	log = log.Session("execrunner")

	log.Info("start")
	defer log.Info("done")

	processPath, err := e.processDepot.CreateProcessDir(log, sandboxHandle, processID)
	if err != nil {
		return nil, err
	}

	specPath := filepath.Join(processPath, "spec.json")
	if err := writeProcessJSON(procJSON, specPath); err != nil {
		return nil, err
	}
	return e.runProcess(log, "exec", []string{"-p", specPath, sandboxHandle}, processID, processPath, pio,
		extraCleanup)
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

func (e *WindowsExecRunner) RunPea(
	log lager.Logger, processID string, processBundle goci.Bndl, sandboxHandle string,
	pio garden.ProcessIO, tty bool, procJSON io.Reader, extraCleanup func() error,
) (garden.Process, error) {
	log = log.Session("execrunner")

	log.Info("start")
	defer log.Info("done")

	processPath, err := e.processDepot.CreateProcessDir(log, sandboxHandle, processID)
	if err != nil {
		return nil, err
	}

	err = e.bundleSaver.Save(processBundle, processPath)
	if err != nil {
		return nil, err
	}

	return e.runProcess(log, "run", []string{"--bundle", processPath, processID}, processID, processPath, pio, extraCleanup)
}

func (e *WindowsExecRunner) runProcess(
	log lager.Logger, runMode string, runtimeExtraArgs []string, processID, processPath string,
	pio garden.ProcessIO, extraCleanup func() error,
) (garden.Process, error) {
	logR, logW, err := os.Pipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating log pipe")
	}
	defer logW.Close()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating stdin pipe")
	}
	defer stdinR.Close()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating stdout pipe")
	}
	defer stdoutW.Close()

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating stderr pipe")
	}
	defer stderrW.Close()

	var childLogW syscall.Handle

	// GetCurrentProcess doesn't error
	self, _ := syscall.GetCurrentProcess()
	// duplicate handle so it is inheritable by child process
	err = syscall.DuplicateHandle(self, syscall.Handle(logW.Fd()), self, &childLogW, 0, true, syscall.DUPLICATE_SAME_ACCESS)
	if err != nil {
		return nil, errors.Wrap(err, "duplicating log pipe handle")
	}

	cmd := exec.Command(e.runtimePath, "--debug", "--log-handle", strconv.FormatUint(uint64(childLogW), 10), "--log-format", "json", runMode, "--pid-file", filepath.Join(processPath, "pidfile"))
	cmd.Args = append(cmd.Args, runtimeExtraArgs...)

	cmd.Stdin = stdinR
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	if err := e.commandRunner.Start(cmd); err != nil {
		return nil, errors.Wrap(err, "execing runtime plugin")
	}

	go streamLogs(log, logR)

	cleanup := func() error {
		e.processMux.Lock()
		delete(e.processes, processID)
		e.processMux.Unlock()

		if extraCleanup != nil {
			return extraCleanup()
		}

		return nil
	}

	proc := &process{
		id:           processID,
		cleanup:      cleanup,
		logger:       log,
		stdin:        stdinW,
		stdout:       stdoutR,
		stderr:       stderrR,
		stdoutWriter: NewDynamicMultiWriter(),
		stderrWriter: NewDynamicMultiWriter(),
		outputWg:     &sync.WaitGroup{},
		exitMutex:    new(sync.RWMutex),
	}

	e.processMux.Lock()
	e.processes[processID] = proc
	e.processMux.Unlock()

	proc.stream(pio, false)

	proc.exitMutex.Lock()

	go func() {
		// the streamLogs go func will only exit once this handle is closed
		defer syscall.CloseHandle(childLogW)
		defer proc.exitMutex.Unlock()
		if err := e.commandRunner.Wait(cmd); err != nil {
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
	}()

	return proc, nil
}

func (e *WindowsExecRunner) Attach(log lager.Logger, _, processID string, pio garden.ProcessIO) (garden.Process, error) {
	proc, err := e.getProcess(processID)
	if err != nil {
		return nil, err
	}

	proc.stream(pio, true)

	return proc, nil
}

func (e *WindowsExecRunner) getProcess(processID string) (*process, error) {
	e.processMux.Lock()
	defer e.processMux.Unlock()

	proc, ok := e.processes[processID]
	if !ok {
		return nil, garden.ProcessNotFoundError{ProcessID: processID}
	}

	return proc, nil
}

func (p *process) stream(pio garden.ProcessIO, duplicate bool) {
	var procStdin *os.File

	procStdin = p.stdin

	if pio.Stdin != nil {
		if duplicate {
			var dupped syscall.Handle
			self, _ := syscall.GetCurrentProcess()
			err := syscall.DuplicateHandle(self, syscall.Handle(p.stdin.Fd()), self, &dupped, 0, false, syscall.DUPLICATE_SAME_ACCESS)
			if err != nil {
				panic(err)
			}

			procStdin = os.NewFile(uintptr(dupped), fmt.Sprintf("%s.stdin", p.id))
		}

		go func() {
			io.Copy(procStdin, pio.Stdin)
			procStdin.Close()
		}()
	}

	if pio.Stdout != nil {
		count := p.stdoutWriter.Attach(pio.Stdout)
		if count == 1 {
			p.outputWg.Add(1)
			go func() {
				io.Copy(p.stdoutWriter, p.stdout)
				p.stdout.Close()
				p.outputWg.Done()
			}()
		}
	}

	if pio.Stderr != nil {
		count := p.stderrWriter.Attach(pio.Stderr)
		if count == 1 {
			p.outputWg.Add(1)
			go func() {
				io.Copy(p.stderrWriter, p.stderr)
				p.stderr.Close()
				p.outputWg.Done()
			}()
		}
	}
}

func (p *process) ID() string {
	return p.id
}

func (p *process) Wait() (int, error) {
	p.exitMutex.RLock()
	defer p.exitMutex.RUnlock()

	p.outputWg.Wait()

	p.stdin.Close()
	p.stdout.Close()
	p.stderr.Close()

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
