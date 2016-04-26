package process_tracker

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/rundmc/iodaemon/link"
	"github.com/cloudfoundry-incubator/guardian/rundmc/process_tracker/writer"
	"github.com/cloudfoundry/gunk/command_runner"
)

type osSignal garden.Signal

func (s osSignal) OsSignal() syscall.Signal {
	switch garden.Signal(s) {
	case garden.SignalTerminate:
		return syscall.SIGTERM
	default:
		return syscall.SIGKILL
	}
}

type Process struct {
	containerPath string
	iodaemonBin   string
	runner        command_runner.CommandRunner
	pidGetter     PidGetter
	id            string
	pidFilePath   string

	runningLink *sync.Once
	linked      chan struct{}
	link        *link.Link

	exited     chan struct{}
	exitStatus int
	exitErr    error

	stdin  writer.FanIn
	stdout writer.FanOut
	stderr writer.FanOut
}

func NewProcess(
	containerPath string,
	iodaemonBin string,
	runner command_runner.CommandRunner,
	pidGetter PidGetter,
	id string,
	pidFilePath string,
) *Process {
	return &Process{
		containerPath: containerPath,
		iodaemonBin:   iodaemonBin,
		runner:        runner,
		pidGetter:     pidGetter,
		id:            id,
		pidFilePath:   pidFilePath,

		runningLink: &sync.Once{},
		linked:      make(chan struct{}),

		exited: make(chan struct{}),

		stdin:  writer.NewFanIn(),
		stdout: writer.NewFanOut(),
		stderr: writer.NewFanOut(),
	}
}

func (p *Process) ID() string {
	return p.id
}

func (p *Process) Wait() (int, error) {
	<-p.exited
	return p.exitStatus, p.exitErr
}

func (p *Process) SetTTY(tty garden.TTYSpec) error {
	<-p.linked

	if tty.WindowSize != nil {
		return p.link.SetWindowSize(tty.WindowSize.Columns, tty.WindowSize.Rows)
	}

	return nil
}

func (p *Process) Signal(signal garden.Signal) error {
	<-p.linked

	pid, err := p.pidGetter.Pid(p.pidFilePath)
	if err != nil {
		return err
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return process.Signal(osSignal(signal).OsSignal())
}

func (p *Process) Spawn(cmd *exec.Cmd, tty *garden.TTYSpec) (ready, active chan error) {
	ready = make(chan error, 1)
	active = make(chan error, 1)

	processSock := path.Join(p.containerPath, "processes", fmt.Sprintf("%s.sock", p.ID()))

	bashFlags := []string{
		"-c",
		// spawn but not as a child process (fork off in the bash subprocess).
		p.iodaemonBin + ` "$@" &`,
		p.iodaemonBin,
	}

	if tty != nil {
		bashFlags = append(bashFlags, "-tty")

		if tty.WindowSize != nil {
			bashFlags = append(
				bashFlags,
				fmt.Sprintf("-windowColumns=%d", tty.WindowSize.Columns),
				fmt.Sprintf("-windowRows=%d", tty.WindowSize.Rows),
			)
		}
	}

	bashFlags = append(bashFlags, "spawn", processSock)

	spawn := exec.Command("bash", append(bashFlags, cmd.Args...)...)
	spawn.Env = cmd.Env
	spawn.Dir = cmd.Dir

	spawnR, err := spawn.StdoutPipe()
	if err != nil {
		ready <- err
		return
	}

	spawnOut := bufio.NewReader(spawnR)

	err = p.runner.Start(spawn)
	if err != nil {
		ready <- err
		return
	}

	go func() {
		_, err := spawnOut.ReadBytes('\n')
		if err != nil {
			ready <- fmt.Errorf("failed to read ready: %s", err)
			return
		}

		ready <- nil

		_, err = spawnOut.ReadBytes('\n')
		if err != nil {
			active <- fmt.Errorf("failed to read active: %s", err)
			return
		}

		active <- nil

		spawn.Wait()
	}()

	return
}

func (p *Process) Link(completionCallback func(processID string)) {
	p.runningLink.Do(func() { p.runLinker(completionCallback) })
}

func (p *Process) Attach(processIO garden.ProcessIO) {
	if processIO.Stdin != nil {
		p.stdin.AddSource(processIO.Stdin)
	}

	if processIO.Stdout != nil {
		p.stdout.AddSink(processIO.Stdout)
	}

	if processIO.Stderr != nil {
		p.stderr.AddSink(processIO.Stderr)
	}
}

// This is guarded by runningLink so will only run once per Process per garden.
func (p *Process) runLinker(cb func(string)) {
	processSock := path.Join(p.containerPath, "processes", fmt.Sprintf("%s.sock", p.ID()))

	defer cb(p.ID())

	link, err := link.Create(processSock, p.stdout, p.stderr)
	if err != nil {
		p.completed(-1, err)
		return
	}

	p.stdin.AddSink(link)

	p.link = link
	close(p.linked)

	p.completed(p.link.Wait())

	// don't leak stdin pipe
	p.stdin.Close()
}

func (p *Process) completed(exitStatus int, err error) {
	p.exitStatus = exitStatus
	p.exitErr = err
	close(p.exited)
}
