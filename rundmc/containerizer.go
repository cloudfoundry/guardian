package rundmc

import (
	"fmt"
	"io"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/logging"
	"github.com/cloudfoundry-incubator/guardian/rundmc/depot"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Depot
//go:generate counterfeiter . BundleGenerator
//go:generate counterfeiter . Checker
//go:generate counterfeiter . BundleRunner
//go:generate counterfeiter . NstarRunner
//go:generate counterfeiter . ContainerStater

type Depot interface {
	Create(log lager.Logger, handle string, bundle depot.BundleSaver) error
	Lookup(log lager.Logger, handle string) (path string, err error)
	Destroy(log lager.Logger, handle string) error
	Handles() ([]string, error)
}

type BundleGenerator interface {
	Generate(spec gardener.DesiredContainerSpec) *goci.Bndl
}

type Checker interface {
	Check(log lager.Logger, output io.Reader) error
}

type ContainerStater interface {
	State(log lager.Logger, id string) (State, error)
}

type BundleRunner interface {
	Start(log lager.Logger, bundlePath, id string, io garden.ProcessIO) (garden.Process, error)
	Exec(log lager.Logger, id, bundlePath string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Kill(log lager.Logger, bundlePath string) error
}

type NstarRunner interface {
	StreamIn(log lager.Logger, pid int, path string, user string, tarStream io.Reader) error
	StreamOut(log lager.Logger, pid int, path string, user string) (io.ReadCloser, error)
}

// Containerizer knows how to manage a depot of container bundles
type Containerizer struct {
	depot        Depot
	bundler      BundleGenerator
	runner       BundleRunner
	startChecker Checker
	stateChecker ContainerStater
	nstar        NstarRunner
}

func New(depot Depot, bundler BundleGenerator, runner BundleRunner, startChecker Checker, stateChecker ContainerStater, nstarRunner NstarRunner) *Containerizer {
	return &Containerizer{
		depot:        depot,
		bundler:      bundler,
		runner:       runner,
		startChecker: startChecker,
		stateChecker: stateChecker,
		nstar:        nstarRunner,
	}
}

// Create creates a bundle in the depot and starts its init process
func (c *Containerizer) Create(log lager.Logger, spec gardener.DesiredContainerSpec) error {
	log = log.Session("containerizer-create", lager.Data{"handle": spec.Handle})

	log.Info("started")
	defer log.Info("finished")

	if err := c.depot.Create(log, spec.Handle, c.bundler.Generate(spec)); err != nil {
		log.Error("create-failed", err)
		return err
	}

	path, err := c.depot.Lookup(log, spec.Handle)
	if err != nil {
		log.Error("lookup-failed", err)
		return err
	}

	stdoutR, stdoutW := io.Pipe()
	_, err = c.runner.Start(log, path, spec.Handle, garden.ProcessIO{
		Stdout: io.MultiWriter(logging.Writer(log), stdoutW),
		Stderr: logging.Writer(log),
	})

	if err != nil {
		log.Error("start", err)
		return err
	}

	if err := c.startChecker.Check(log, stdoutR); err != nil {
		log.Error("check", err)
		return err
	}

	_, err = c.stateChecker.State(log, spec.Handle)
	if err != nil {
		log.Error("check-state-failed", err)
		return fmt.Errorf("create: state file not found for container")
	}

	return nil
}

// Run runs a process inside a running container
func (c *Containerizer) Run(log lager.Logger, handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("run", lager.Data{"handle": handle, "path": spec.Path})

	log.Info("started")
	defer log.Info("finished")

	path, err := c.depot.Lookup(log, handle)
	if err != nil {
		log.Error("lookup", err)
		return nil, err
	}

	return c.runner.Exec(log, path, handle, spec, io)
}

// StreamIn streams files in to the container
func (c *Containerizer) StreamIn(log lager.Logger, handle string, spec garden.StreamInSpec) error {
	log = log.Session("stream-in", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	state, err := c.stateChecker.State(log, handle)
	if err != nil {
		log.Error("check-pid-failed", err)
		return fmt.Errorf("stream-in: pid not found for container")
	}

	if err := c.nstar.StreamIn(log, state.Pid, spec.Path, spec.User, spec.TarStream); err != nil {
		log.Error("nstar-failed", err)
		return fmt.Errorf("stream-in: nstar: %s", err)
	}

	return nil
}

// StreamOut stream files from the container
func (c *Containerizer) StreamOut(log lager.Logger, handle string, spec garden.StreamOutSpec) (io.ReadCloser, error) {
	log = log.Session("stream-out", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	state, err := c.stateChecker.State(log, handle)
	if err != nil {
		log.Error("check-pid-failed", err)
		return nil, fmt.Errorf("stream-out: pid not found for container")
	}

	stream, err := c.nstar.StreamOut(log, state.Pid, spec.Path, spec.User)
	if err != nil {
		log.Error("nstar-failed", err)
		return nil, fmt.Errorf("stream-out: nstar: %s", err)
	}

	return stream, nil
}

// Destroy kills any container processes and deletes the bundle directory
func (c *Containerizer) Destroy(log lager.Logger, handle string) error {
	log = log.Session("destroy", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	_, err := c.stateChecker.State(log, handle)
	if err != nil {
		log.Error("pid-gone-skip-kill", err)
		return c.depot.Destroy(log, handle)
	}

	if err := c.runner.Kill(log, handle); err != nil {
		log.Error("kill-failed", err)
		return err
	}

	return c.depot.Destroy(log, handle)
}

func (c *Containerizer) Info(log lager.Logger, handle string) (gardener.ActualContainerSpec, error) {
	bundlePath, err := c.depot.Lookup(log, handle)

	if err != nil {
		return gardener.ActualContainerSpec{}, err
	}

	return gardener.ActualContainerSpec{
		BundlePath: bundlePath,
	}, nil
}

// Handles returns a list of all container handles
func (c *Containerizer) Handles() ([]string, error) {
	return c.depot.Handles()
}
