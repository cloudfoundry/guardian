package runrunc

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci/specs"
	"github.com/cloudfoundry-incubator/guardian/log"
	"github.com/cloudfoundry-incubator/guardian/rundmc/process_tracker"
	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/pivotal-golang/lager"
)

const DefaultPath = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

var plog = log.Session("runrunc")

//go:generate counterfeiter . ProcessTracker
type ProcessTracker interface {
	Run(id string, cmd *exec.Cmd, io garden.ProcessIO, tty *garden.TTYSpec, signaller process_tracker.Signaller) (garden.Process, error)
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

	log log.ChainLogger
}

//go:generate counterfeiter . RuncBinary
type RuncBinary interface {
	StartCommand(path, id string) *exec.Cmd
	ExecCommand(id, processJSONPath string) *exec.Cmd
}

func New(tracker ProcessTracker, runner command_runner.CommandRunner, pidgen UidGenerator, runc RuncBinary) *RunRunc {
	return &RunRunc{
		tracker:       tracker,
		commandRunner: runner,
		pidGenerator:  pidgen,
		runc:          runc,

		log: plog,
	}
}

func (r RunRunc) WithLogSession(sess log.ChainLogger) *RunRunc {
	var cp RunRunc = r
	r.log = sess.Start("runrunc")
	r.commandRunner = &log.Runner{CommandRunner: r.commandRunner, Logger: r.log}

	return &cp
}

// Starts a bundle by running 'runc' in the bundle directory
func (r *RunRunc) Start(bundlePath, id string, io garden.ProcessIO) (garden.Process, error) {
	mlog := plog.Start("start", lager.Data{"bundle": bundlePath})

	cmd := r.runc.StartCommand(bundlePath, id)

	process, err := r.tracker.Run(r.pidGenerator.Generate(), cmd, io, nil, nil)
	if err != nil {
		return nil, mlog.Err("run", err)
	}

	mlog.Info("started")
	return process, nil
}

// Exec a process in a bundle using 'runc exec'
func (r *RunRunc) Exec(id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	mlog := plog.Start("exec", lager.Data{"id": id, "path": spec.Path})

	tmpFile, err := ioutil.TempFile("", "guardianprocess")
	if err != nil {
		return nil, mlog.Err("tempfile", err)
	}

	if err := writeProcessJSON(spec, tmpFile); err != nil {
		return nil, mlog.Err("encode", err)
	}

	cmd := r.runc.ExecCommand(id, tmpFile.Name())

	process, err := r.tracker.Run(r.pidGenerator.Generate(), cmd, io, spec.TTY, nil)
	if err != nil {
		return nil, mlog.Err("run", err)
	}

	mlog.Info("execed")
	return process, nil
}

// Kill a bundle using 'runc kill'
func (r *RunRunc) Kill(bundlePath string) error {
	mlog := plog.Start("kill", lager.Data{"bundle": bundlePath})

	cmd := exec.Command("runc", "kill", "SIGKILL")
	cmd.Dir = bundlePath
	err := r.commandRunner.Run(cmd)
	if err != nil {
		return mlog.Err("run", err)
	}

	mlog.Info("killed")
	return err
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
