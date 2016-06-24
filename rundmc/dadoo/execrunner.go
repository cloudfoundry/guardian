package dadoo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/kr/logfmt"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . PidGetter
type PidGetter interface {
	Pid(pidFilePath string) (int, error)
}

type ExecRunner struct {
	dadooPath      string
	runcPath       string
	processIDGen   runrunc.UidGenerator
	pidGetter      PidGetter
	iodaemonRunner runrunc.ExecRunner
	commandRunner  command_runner.CommandRunner
}

func NewExecRunner(dadooPath, runcPath string, processIDGen runrunc.UidGenerator, pidGetter PidGetter, iodaemonRunner runrunc.ExecRunner, commandRunner command_runner.CommandRunner) *ExecRunner {
	return &ExecRunner{
		dadooPath:      dadooPath,
		runcPath:       runcPath,
		processIDGen:   processIDGen,
		pidGetter:      pidGetter,
		iodaemonRunner: iodaemonRunner,
		commandRunner:  commandRunner,
	}
}

func (d *ExecRunner) Run(log lager.Logger, spec *runrunc.PreparedSpec, processesPath, handle string, tty *garden.TTYSpec, pio garden.ProcessIO) (p garden.Process, theErr error) {
	if !contains(spec.Env, "USE_DADOO=true") {
		return d.iodaemonRunner.Run(log, spec, processesPath, handle, tty, pio)
	}

	log = log.Session("execrunner")
	log.Info("start")
	defer log.Info("done")

	processID := d.processIDGen.Generate()
	processPath := filepath.Join(processesPath, processID)

	encodedSpec, err := json.Marshal(spec.Process)
	if err != nil {
		return nil, err // this could *almost* be a panic: a valid spec should always encode (but out of caution we'll error)
	}

	if err := os.MkdirAll(processPath, 0700); err != nil {
		return nil, err
	}

	pipes, err := mkFifos(pio, filepath.Join(processPath, "stdin"), filepath.Join(processPath, "stdout"), filepath.Join(processPath, "stderr"))
	if err != nil {
		return nil, err
	}

	fd3r, fd3w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	logr, logw, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	winszr, winszw, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	defer fd3r.Close()
	defer logr.Close()

	var cmd *exec.Cmd
	if tty != nil {
		cmd = exec.Command(d.dadooPath, "-tty", "-uid", strconv.Itoa(spec.HostUID), "-gid", strconv.Itoa(spec.HostGID), "exec", d.runcPath, processPath, handle)
		sendWindowSize(winszw, tty.WindowSize)
	} else {
		cmd = exec.Command(d.dadooPath, "exec", d.runcPath, processPath, handle)
	}

	cmd.Stdin = bytes.NewReader(encodedSpec)
	cmd.ExtraFiles = []*os.File{
		fd3w,
		logw,
		winszr,
	}

	if err := d.commandRunner.Start(cmd); err != nil {
		return nil, err
	}

	fd3w.Close()
	logw.Close()
	winszr.Close()

	log.Info("open-pipes")

	if err := pipes.start(); err != nil {
		return nil, err
	}

	log.Info("read-exit-fd")

	runcExitStatus := make([]byte, 1)
	fd3r.Read(runcExitStatus)

	log.Info("runc-exit-status", lager.Data{"status": runcExitStatus[0]})

	defer func() {
		theErr = processLogs(log, logr, theErr)
	}()

	if runcExitStatus[0] != 0 {
		return nil, fmt.Errorf("exit status %d", runcExitStatus[0])
	}

	return d.newProcess(cmd, filepath.Join(processPath, "pidfile"), winszw), nil
}

func sendWindowSize(winszw io.Writer, winSize *garden.WindowSize) {
	if winSize == nil {
		return
	}

	initialSize := TtySize{
		Cols: uint16(winSize.Columns),
		Rows: uint16(winSize.Rows),
	}
	json.NewEncoder(winszw).Encode(initialSize)
}

func (d *ExecRunner) Attach(log lager.Logger, processID string, io garden.ProcessIO, processesPath string) (garden.Process, error) {
	return d.iodaemonRunner.Attach(log, processID, io, processesPath)
}

type osSignal garden.Signal

func (s osSignal) OsSignal() syscall.Signal {
	switch garden.Signal(s) {
	case garden.SignalTerminate:
		return syscall.SIGTERM
	default:
		return syscall.SIGKILL
	}
}

type process struct {
	pidFilePath   string
	pidGetter     PidGetter
	wait          func() error
	winSizeWriter io.WriteCloser
}

func (d *ExecRunner) newProcess(cmd *exec.Cmd, pidFilePath string, winszw io.WriteCloser) *process {
	exitCh := make(chan struct{})
	var exitErr error
	go func() {
		exitErr = d.commandRunner.Wait(cmd)
		close(exitCh)
	}()

	return &process{
		wait: func() error {
			<-exitCh
			return exitErr
		},
		pidFilePath:   pidFilePath,
		pidGetter:     d.pidGetter,
		winSizeWriter: winszw,
	}
}

func (p *process) ID() string {
	return ""
}

func (p *process) Wait() (int, error) {
	defer p.winSizeWriter.Close()

	if err := p.wait(); err != nil {
		exitError, ok := err.(ExitError)
		if !ok {
			return 255, err
		}

		waitStatus, ok := exitError.Sys().(ExitStatuser)
		if !ok {
			return 255, err
		}

		return waitStatus.ExitStatus(), nil
	}

	return 0, nil
}

func (p *process) SetTTY(tty garden.TTYSpec) error {
	sendWindowSize(p.winSizeWriter, tty.WindowSize)
	return nil
}

func (p *process) Signal(signal garden.Signal) error {
	pid, err := p.pidGetter.Pid(p.pidFilePath)
	if err != nil {
		return errors.New(fmt.Sprintf("fetching-pid: %s", err))
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return errors.New(fmt.Sprintf("finding-process: %s", err))
	}

	return process.Signal(osSignal(signal).OsSignal())
}

type ExitError interface {
	Sys() interface{}
}

type ExitStatuser interface {
	ExitStatus() int
}

type fifos [3]struct {
	Name     string
	Path     string
	CopyTo   io.Writer
	CopyFrom io.Reader
	Open     func(p string) (*os.File, error)
}

func mkFifos(pio garden.ProcessIO, stdin, stdout, stderr string) (fifos, error) {
	pipes := fifos{
		{Name: "stdin", Path: stdin, CopyFrom: pio.Stdin, Open: func(p string) (*os.File, error) { return os.OpenFile(p, os.O_WRONLY, 0600) }},
		{Name: "stdout", Path: stdout, CopyTo: pio.Stdout, Open: os.Open},
		{Name: "stderr", Path: stderr, CopyTo: pio.Stderr, Open: os.Open},
	}

	for _, pipe := range pipes {
		if err := syscall.Mkfifo(pipe.Path, 0); err != nil {
			return pipes, err
		}
	}

	return pipes, nil
}

func (f fifos) start() error {
	for _, pipe := range f {
		r, err := pipe.Open(pipe.Path)
		if err != nil {
			return err
		}

		if pipe.CopyFrom != nil {
			go io.Copy(r, pipe.CopyFrom)
		}

		if pipe.CopyTo != nil {
			go io.Copy(pipe.CopyTo, r)
		}
	}

	return nil
}

func contains(envVars []string, envVar string) bool {
	for _, e := range envVars {
		if e == envVar {
			return true
		}
	}
	return false
}

func processLogs(log lager.Logger, logs io.Reader, err error) error {
	buff, readErr := ioutil.ReadAll(logs)
	if readErr != nil {
		return fmt.Errorf("start: read log file: %s", readErr)
	}

	forwardRuncLogsToLager(log, buff)

	if err != nil {
		return wrapWithErrorFromRuncLog(log, err, buff)
	}

	return nil
}

func forwardRuncLogsToLager(log lager.Logger, buff []byte) {
	parsedLogLine := struct{ Msg string }{}
	for _, logLine := range strings.Split(string(buff), "\n") {
		if err := logfmt.Unmarshal([]byte(logLine), &parsedLogLine); err == nil {
			log.Debug("runc", lager.Data{
				"message": parsedLogLine.Msg,
			})
		}
	}
}

func wrapWithErrorFromRuncLog(log lager.Logger, originalError error, buff []byte) error {
	parsedLogLine := struct{ Msg string }{}
	logfmt.Unmarshal(buff, &parsedLogLine)
	return fmt.Errorf("runc exec: %s: %s", originalError, parsedLogLine.Msg)
}
