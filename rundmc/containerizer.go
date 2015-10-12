package rundmc

import (
	"io"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/logging"
	"github.com/cloudfoundry-incubator/guardian/rundmc/depot"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Depot
type Depot interface {
	Create(log lager.Logger, handle string, bundle depot.BundleSaver) error
	Lookup(log lager.Logger, handle string) (path string, err error)
	Destroy(log lager.Logger, handle string) error
}

//go:generate counterfeiter . Bundler
type Bundler interface {
	Bundle(spec gardener.DesiredContainerSpec) *goci.Bndl
}

//go:generate counterfeiter . Checker
type Checker interface {
	Check(log lager.Logger, output io.Reader) error
}

//go:generate counterfeiter . BundleRunner
type BundleRunner interface {
	Start(log lager.Logger, bundlePath, id string, io garden.ProcessIO) (garden.Process, error)
	Exec(log lager.Logger, id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Kill(log lager.Logger, bundlePath string) error
}

// Containerizer knows how to manage a depot of container bundles
type Containerizer struct {
	depot        Depot
	bundler      Bundler
	runner       BundleRunner
	startChecker Checker
}

func New(depot Depot, bundler Bundler, runner BundleRunner, startChecker Checker) *Containerizer {
	return &Containerizer{
		depot:        depot,
		bundler:      bundler,
		runner:       runner,
		startChecker: startChecker,
	}
}

// Create creates a bundle in the depot and starts its init process
func (c *Containerizer) Create(log lager.Logger, spec gardener.DesiredContainerSpec) error {
	log = log.Session("create", lager.Data{"handle": spec.Handle})

	log.Info("started")
	defer log.Info("finished")

	if err := c.depot.Create(log, spec.Handle, c.bundler.Bundle(spec)); err != nil {
		log.Error("create-failed", err)
		return err
	}

	path, err := c.depot.Lookup(log, spec.Handle)
	if err != nil {
		log.Error("lookup-failed", err)
		return err
	}

	stdoutR, stdoutW := io.Pipe()
	if c.runner.Start(log, path, spec.Handle, garden.ProcessIO{
		Stdout: io.MultiWriter(logging.Writer(log), stdoutW),
		Stderr: logging.Writer(log),
	}); err != nil {
		log.Error("start", err)
		return err
	}

	if err := c.startChecker.Check(log, stdoutR); err != nil {
		log.Error("check", err)
		return err
	}

	return nil
}

// Run runs a process inside a running container
func (c *Containerizer) Run(log lager.Logger, handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("run", lager.Data{"handle": handle, "path": spec.Path})

	log.Info("started")
	defer log.Info("finished")

	_, err := c.depot.Lookup(log, handle)
	if err != nil {
		log.Error("lookup", err)
		return nil, err
	}

	return c.runner.Exec(log, handle, spec, io)
}

// Destroy kills any container processes and deletes the bundle directory
func (c *Containerizer) Destroy(log lager.Logger, handle string) error {
	log = log.Session("destroy", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	path, err := c.depot.Lookup(log, handle)
	if err != nil {
		log.Error("lookup-failed", err)
		return err
	}

	if err := c.runner.Kill(log, path); err != nil {
		log.Error("kill-failed", err)
		return err
	}

	return c.depot.Destroy(log, handle)
}
