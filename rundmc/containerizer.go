package rundmc

import (
	"io"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/log"
	"github.com/cloudfoundry-incubator/guardian/rundmc/depot"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	"github.com/pivotal-golang/lager"
)

var plog = log.Session("rundmc")

//go:generate counterfeiter . Depot
type Depot interface {
	Create(handle string, bundle depot.BundleSaver) error
	Lookup(handle string) (path string, err error)
	Destroy(handle string) error
}

//go:generate counterfeiter . Bundler
type Bundler interface {
	Bundle(spec gardener.DesiredContainerSpec) *goci.Bndl
}

//go:generate counterfeiter . Checker
type Checker interface {
	Check(output io.Reader) error
}

//go:generate counterfeiter . BundleRunner
type BundleRunner interface {
	Start(bundlePath, id string, io garden.ProcessIO) (garden.Process, error)
	Exec(id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Kill(bundlePath string) error
}

// Containerizer knows how to manage a depot of container bundles
type Containerizer struct {
	depot        Depot
	bundler      Bundler
	runner       *MaybeLoggingRunner
	startChecker Checker
}

func New(depot Depot, bundler Bundler, runner BundleRunner, startChecker Checker) *Containerizer {
	return &Containerizer{
		depot:        depot,
		bundler:      bundler,
		runner:       &MaybeLoggingRunner{runner},
		startChecker: startChecker,
	}
}

// Create creates a bundle in the depot and starts its init process
func (c *Containerizer) Create(spec gardener.DesiredContainerSpec) error {
	mlog := plog.Start("create", lager.Data{"handle": spec.Handle})
	defer mlog.Info("created")

	if err := c.depot.Create(spec.Handle, c.bundler.Bundle(spec)); err != nil {
		return mlog.Err("create", err)
	}

	path, err := c.depot.Lookup(spec.Handle)
	if err != nil {
		return mlog.Err("lookup", err)
	}

	stdoutR, stdoutW := io.Pipe()
	if c.runner.withLog(mlog).Start(path, spec.Handle, garden.ProcessIO{
		Stdout: io.MultiWriter(mlog.Start("start-stdout"), stdoutW),
		Stderr: mlog.Start("start-stderr"),
	}); err != nil {
		return mlog.Err("start", err)
	}

	if err := c.startChecker.Check(stdoutR); err != nil {
		return mlog.Err("check", err)
	}

	return nil
}

// Run runs a process inside a running container
func (c *Containerizer) Run(handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	mlog := plog.Start("run", lager.Data{"handle": handle, "path": spec.Path})
	defer mlog.Info("ran")

	_, err := c.depot.Lookup(handle)
	if err != nil {
		return nil, mlog.Err("lookup", err)
	}

	return c.runner.withLog(mlog).Exec(handle, spec, io)
}

// Destroy kills any container processes and deletes the bundle directory
func (c *Containerizer) Destroy(handle string) error {
	mlog := plog.Start("destroy", lager.Data{"handle": handle})
	defer mlog.Info("destroyed")

	path, err := c.depot.Lookup(handle)
	if err != nil {
		return mlog.Err("lookup", err)
	}

	if err := c.runner.withLog(mlog).Kill(path); err != nil {
		return mlog.Err("kill", err)
	}

	return c.depot.Destroy(handle)
}

type MaybeLoggingRunner struct{ BundleRunner }

func (m MaybeLoggingRunner) withLog(log log.ChainLogger) BundleRunner {
	switch r := m.BundleRunner.(type) {
	case *runrunc.RunRunc:
		return r.WithLogSession(log)
	default:
		return r
	}
}
