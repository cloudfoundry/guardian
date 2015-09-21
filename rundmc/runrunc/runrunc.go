package runrunc

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/log"
	"github.com/cloudfoundry-incubator/guardian/rundmc/process_tracker"
	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/opencontainers/specs"
	"github.com/pivotal-golang/lager"
)

const DefaultPath = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

var plog = log.Session("runrunc")

//go:generate counterfeiter . ProcessTracker
type ProcessTracker interface {
	Run(id uint32, cmd *exec.Cmd, io garden.ProcessIO, tty *garden.TTYSpec, signaller process_tracker.Signaller) (garden.Process, error)
}

//go:generate counterfeiter . PidGenerator
type PidGenerator interface {
	Generate() uint32
}

// da doo
type RunRunc struct {
	tracker       ProcessTracker
	commandRunner command_runner.CommandRunner
	pidGenerator  PidGenerator

	log log.ChainLogger
}

func New(tracker ProcessTracker, runner command_runner.CommandRunner, pidgen PidGenerator) *RunRunc {
	return &RunRunc{
		tracker:       tracker,
		commandRunner: runner,
		pidGenerator:  pidgen,

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
func (r *RunRunc) Start(bundlePath string, io garden.ProcessIO) (garden.Process, error) {
	mlog := plog.Start("start", lager.Data{"bundle": bundlePath})
	defer mlog.Info("started")

	cmd := exec.Command("runc")
	cmd.Dir = bundlePath

	process, err := r.tracker.Run(r.pidGenerator.Generate(), cmd, io, &garden.TTYSpec{}, nil)
	return process, err
}

// Exec a process in a bundle using 'runc exec'
func (r *RunRunc) Exec(bundlePath string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	mlog := plog.Start("exec", lager.Data{"bundle": bundlePath, "path": spec.Path})
	defer mlog.Info("execced")

	tmpFile, err := ioutil.TempFile("", "guardianprocess")
	if err != nil {
		return nil, mlog.Err("tempfile", err)
	}

	if err := writeProcessJSON(spec, tmpFile); err != nil {
		return nil, mlog.Err("encode", err)
	}

	cmd := exec.Command("runc", "exec", tmpFile.Name())
	cmd.Dir = bundlePath

	return r.tracker.Run(r.pidGenerator.Generate(), cmd, io, spec.TTY, nil)
}

// Kill a bundle using 'runc kill'
func (r *RunRunc) Kill(bundlePath string) error {
	mlog := plog.Start("kill", lager.Data{"bundle": bundlePath})
	defer mlog.Info("killed")

	cmd := exec.Command("runc", "kill")
	cmd.Dir = bundlePath
	return r.commandRunner.Run(cmd)
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
