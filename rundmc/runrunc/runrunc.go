package runrunc

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/pivotal-golang/lager"
)

const DefaultRootPath = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
const DefaultPath = "PATH=/usr/local/bin:/usr/bin:/bin"

//go:generate counterfeiter . ProcessTracker
type ProcessTracker interface {
	Run(id string, cmd *exec.Cmd, io garden.ProcessIO, tty *garden.TTYSpec, pidFile string) (garden.Process, error)
}

//go:generate counterfeiter . UidGenerator
type UidGenerator interface {
	Generate() string
}

//go:generate counterfeiter . UserLookupper
type UserLookupper interface {
	Lookup(rootFsPath string, user string) (*user.ExecUser, error)
}

//go:generate counterfeiter . Mkdirer
type Mkdirer interface {
	MkdirAs(path string, mode os.FileMode, uid, gid int) error
}

type LookupFunc func(rootfsPath, user string) (*user.ExecUser, error)

func (fn LookupFunc) Lookup(rootfsPath, user string) (*user.ExecUser, error) {
	return fn(rootfsPath, user)
}

//go:generate counterfeiter . BundleLoader
type BundleLoader interface {
	Load(path string) (*goci.Bndl, error)
}

// da doo
type RunRunc struct {
	tracker       ProcessTracker
	commandRunner command_runner.CommandRunner
	pidGenerator  UidGenerator
	runc          RuncBinary

	execPreparer *ExecPreparer
}

//go:generate counterfeiter . RuncBinary
type RuncBinary interface {
	StartCommand(path, id string) *exec.Cmd
	ExecCommand(id, processJSONPath, pidFilePath string) *exec.Cmd
	KillCommand(id, signal string) *exec.Cmd
}

func New(tracker ProcessTracker, runner command_runner.CommandRunner, pidgen UidGenerator, runc RuncBinary, execPreparer *ExecPreparer) *RunRunc {
	return &RunRunc{
		tracker:       tracker,
		commandRunner: runner,
		pidGenerator:  pidgen,
		runc:          runc,
		execPreparer:  execPreparer,
	}
}

// Starts a bundle by running 'runc' in the bundle directory
func (r *RunRunc) Start(log lager.Logger, bundlePath, id string, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("start", lager.Data{"bundle": bundlePath})

	log.Info("started")
	defer log.Info("finished")

	cmd := r.runc.StartCommand(bundlePath, id)

	process, err := r.tracker.Run(r.pidGenerator.Generate(), cmd, io, nil, "")
	if err != nil {
		log.Error("run", err)
		return nil, err
	}

	return process, nil
}

// Exec a process in a bundle using 'runc exec'
func (r *RunRunc) Exec(log lager.Logger, bundlePath, id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("exec", lager.Data{"id": id, "path": spec.Path})

	pid := r.pidGenerator.Generate()

	log.Info("started", lager.Data{pid: "pid"})
	defer log.Info("finished")

	pidFilePath := path.Join(bundlePath, "processes", fmt.Sprintf("%s.pid", pid))
	cmd, err := r.execPreparer.Prepare(log, id, bundlePath, pidFilePath, spec, r.runc)
	if err != nil {
		log.Error("prepare-failed", err)
		return nil, err
	}

	process, err := r.tracker.Run(pid, cmd, io, spec.TTY, pidFilePath)
	if err != nil {
		log.Error("run-failed", err)
		return nil, err
	}

	return process, nil
}

// Kill a bundle using 'runc kill'
func (r *RunRunc) Kill(log lager.Logger, handle string) error {
	log = log.Session("kill", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	buf := &bytes.Buffer{}
	cmd := r.runc.KillCommand(handle, "KILL")
	cmd.Stderr = buf
	if err := r.commandRunner.Run(cmd); err != nil {
		log.Error("run-failed", err, lager.Data{"stderr": buf.String()})
		return fmt.Errorf("runc kill: %s: %s", err, string(buf.String()))
	}

	return nil
}
