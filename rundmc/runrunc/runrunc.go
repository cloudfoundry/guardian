package runrunc

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci/specs"
	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/pivotal-golang/lager"
)

const DefaultPath = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

//go:generate counterfeiter . ProcessTracker
type ProcessTracker interface {
	Run(id string, cmd *exec.Cmd, io garden.ProcessIO, tty *garden.TTYSpec) (garden.Process, error)
}

//go:generate counterfeiter . UidGenerator
type UidGenerator interface {
	Generate() string
}

// da doo
type RunRunc struct {
	tracker       ProcessTracker
	commandRunner command_runner.CommandRunner
	pidGenerator  UidGenerator
	runc          RuncBinary
}

//go:generate counterfeiter . RuncBinary
type RuncBinary interface {
	StartCommand(path, id string) *exec.Cmd
	ExecCommand(id, processJSONPath string) *exec.Cmd
	KillCommand(id, signal string) *exec.Cmd
}

func New(tracker ProcessTracker, runner command_runner.CommandRunner, pidgen UidGenerator, runc RuncBinary) *RunRunc {
	return &RunRunc{
		tracker:       tracker,
		commandRunner: runner,
		pidGenerator:  pidgen,
		runc:          runc,
	}
}

// Starts a bundle by running 'runc' in the bundle directory
func (r *RunRunc) Start(log lager.Logger, bundlePath, id string, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("start", lager.Data{"bundle": bundlePath})

	log.Info("started")
	defer log.Info("finished")

	cmd := r.runc.StartCommand(bundlePath, id)

	process, err := r.tracker.Run(r.pidGenerator.Generate(), cmd, io, nil)
	if err != nil {
		log.Error("run", err)
		return nil, err
	}

	return process, nil
}

// Exec a process in a bundle using 'runc exec'
func (r *RunRunc) Exec(log lager.Logger, id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("exec", lager.Data{"id": id, "path": spec.Path})

	log.Info("started")
	defer log.Info("finished")

	tmpFile, err := ioutil.TempFile("", "guardianprocess")
	if err != nil {
		log.Error("tempfile-failed", err)
		return nil, err
	}

	if err := writeProcessJSON(spec, tmpFile); err != nil {
		log.Error("encode-failed", err)
		return nil, err
	}

	cmd := r.runc.ExecCommand(id, tmpFile.Name())

	process, err := r.tracker.Run(r.pidGenerator.Generate(), cmd, io, spec.TTY)
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

	if err := r.commandRunner.Run(r.runc.KillCommand(handle, "KILL")); err != nil {
		log.Error("run-failed", err)
		return err
	}

	return nil
}

func writeProcessJSON(spec garden.ProcessSpec, writer io.Writer) error {
	return json.NewEncoder(writer).Encode(specs.Process{
		Args: append([]string{spec.Path}, spec.Args...),
		Env:  envWithPath(spec.Env),
	})
}

func envWithPath(env []string) []string {
	for _, envVar := range env {
		if strings.Contains(envVar, "PATH=") {
			return env
		}
	}

	return append(env, DefaultPath)
}
