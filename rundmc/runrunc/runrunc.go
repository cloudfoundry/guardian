package runrunc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/opencontainers/specs"
	"github.com/pivotal-golang/lager"
)

const DefaultRootPath = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
const DefaultPath = "PATH=/usr/local/bin:/usr/bin:/bin"

//go:generate counterfeiter . ProcessTracker
type ProcessTracker interface {
	Run(id string, cmd *exec.Cmd, io garden.ProcessIO, tty *garden.TTYSpec) (garden.Process, error)
}

//go:generate counterfeiter . UidGenerator
type UidGenerator interface {
	Generate() string
}

//go:generate counterfeiter . UserLookupper
type UserLookupper interface {
	Lookup(rootFsPath string, user string) (uint32, uint32, error)
}

type LookupFunc func(rootfsPath, user string) (uint32, uint32, error)

func (fn LookupFunc) Lookup(rootfsPath, user string) (uint32, uint32, error) {
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

	bundleLoader BundleLoader
	users        UserLookupper
}

//go:generate counterfeiter . RuncBinary
type RuncBinary interface {
	StartCommand(path, id string) *exec.Cmd
	ExecCommand(id, processJSONPath string) *exec.Cmd
	KillCommand(id, signal string) *exec.Cmd
}

func New(tracker ProcessTracker, runner command_runner.CommandRunner, pidgen UidGenerator, runc RuncBinary, bundleLoader BundleLoader, users UserLookupper) *RunRunc {
	return &RunRunc{
		tracker:       tracker,
		commandRunner: runner,
		pidGenerator:  pidgen,
		runc:          runc,
		bundleLoader:  bundleLoader,
		users:         users,
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
func (r *RunRunc) Exec(log lager.Logger, bundlePath, id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("exec", lager.Data{"id": id, "path": spec.Path})

	log.Info("started")
	defer log.Info("finished")

	tmpFile, err := ioutil.TempFile("", "guardianprocess")
	if err != nil {
		log.Error("tempfile-failed", err)
		return nil, err
	}

	if err := r.writeProcessJSON(bundlePath, spec, tmpFile); err != nil {
		log.Error("encode-failed", err)
		return nil, fmt.Errorf("writeProcessJSON for container %s: %s", id, err)
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

	buf := &bytes.Buffer{}
	cmd := r.runc.KillCommand(handle, "KILL")
	cmd.Stderr = buf
	if err := r.commandRunner.Run(cmd); err != nil {
		log.Error("run-failed", err, lager.Data{"stderr": buf.String()})
		return fmt.Errorf("runc kill: %s: %s", err, string(buf.String()))
	}

	return nil
}

func (r *RunRunc) writeProcessJSON(bundlePath string, spec garden.ProcessSpec, writer io.Writer) error {
	bndl, err := r.bundleLoader.Load(bundlePath)
	if err != nil {
		return err
	}

	rootFsPath := bndl.RootFS()
	if rootFsPath == "" {
		return fmt.Errorf("empty rootfs path")
	}

	uid, gid, err := r.users.Lookup(rootFsPath, spec.User)
	if err != nil {
		return err
	}

	defaultPath := DefaultPath
	if uid == 0 {
		defaultPath = DefaultRootPath
	}

	env := envWithDefaultPath(append(
		bndl.Spec.Spec.Process.Env, spec.Env...,
	), defaultPath)
	return json.NewEncoder(writer).Encode(specs.Process{
		Args: append([]string{spec.Path}, spec.Args...),
		Env:  env,
		User: specs.User{
			UID: uid,
			GID: gid,
		},
	})
}

func envWithDefaultPath(env []string, defaultPath string) []string {
	for _, envVar := range env {
		if strings.Contains(envVar, "PATH=") {
			return env
		}
	}

	return append(env, defaultPath)
}
