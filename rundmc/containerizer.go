package rundmc

import (
	"fmt"
	"io"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc/depot"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Depot
//go:generate counterfeiter . BundleGenerator
//go:generate counterfeiter . BundleRunner
//go:generate counterfeiter . NstarRunner
//go:generate counterfeiter . EventStore
//go:generate counterfeiter . BundleLoader
//go:generate counterfeiter . ExitStore
//go:generate counterfeiter . Stopper

type Depot interface {
	Create(log lager.Logger, handle string, bundle depot.BundleSaver) error
	Lookup(log lager.Logger, handle string) (path string, err error)
	Destroy(log lager.Logger, handle string) error
	Handles() ([]string, error)
}

type BundleGenerator interface {
	Generate(spec gardener.DesiredContainerSpec) *goci.Bndl
}

type BundleLoader interface {
	Load(path string) (*goci.Bndl, error)
}

type BundleRunner interface {
	Start(log lager.Logger, bundlePath, id string, io garden.ProcessIO) (exit <-chan struct{}, err error)
	Exec(log lager.Logger, id, bundlePath string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Kill(log lager.Logger, bundlePath string) error
	State(log lager.Logger, id string) (runrunc.State, error)
	Stats(log lager.Logger, id string) (gardener.ActualContainerMetrics, error)
	WatchEvents(log lager.Logger, id string, eventsNotifier runrunc.EventsNotifier) error
}

type NstarRunner interface {
	StreamIn(log lager.Logger, pid int, path string, user string, tarStream io.Reader) error
	StreamOut(log lager.Logger, pid int, path string, user string) (io.ReadCloser, error)
}

type Stopper interface {
	StopAll(log lager.Logger, cgroupName string, save []int, kill bool) error
}

type EventStore interface {
	OnEvent(id string, event string) error
	Events(id string) []string
}

type ExitStore interface {
	Store(handle string, exit <-chan struct{})
	Unstore(handle string)
	Wait(handle string)
}

// Containerizer knows how to manage a depot of container bundles
type Containerizer struct {
	depot   Depot
	bundler BundleGenerator
	loader  BundleLoader
	runner  BundleRunner
	stopper Stopper
	nstar   NstarRunner
	exits   ExitStore
	events  EventStore
}

func New(depot Depot, bundler BundleGenerator, runner BundleRunner, loader BundleLoader, nstarRunner NstarRunner, stopper Stopper, exitStore ExitStore, events EventStore) *Containerizer {
	return &Containerizer{
		depot:   depot,
		bundler: bundler,
		runner:  runner,
		loader:  loader,
		nstar:   nstarRunner,
		stopper: stopper,
		events:  events,
		exits:   exitStore,
	}
}

// Create creates a bundle in the depot and starts its init process
func (c *Containerizer) Create(log lager.Logger, spec gardener.DesiredContainerSpec) error {
	log = log.Session("containerizer-create", lager.Data{"handle": spec.Handle})

	log.Info("start")
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

	exitCh, err := c.runner.Start(log, path, spec.Handle, garden.ProcessIO{})
	if err != nil {
		log.Error("start", err)
		return err
	}

	c.exits.Store(spec.Handle, exitCh)

	go func() {
		if err := c.runner.WatchEvents(log, spec.Handle, c.events); err != nil {
			log.Error("watch-failed", err)
		}
	}()

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

	state, err := c.runner.State(log, handle)
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

	state, err := c.runner.State(log, handle)
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

// Stop stops all the processes other than the init process in the container
func (c *Containerizer) Stop(log lager.Logger, handle string, kill bool) error {
	log = log.Session("stop", lager.Data{"handle": handle, "kill": kill})

	log.Info("started")
	defer log.Info("finished")

	state, err := c.runner.State(log, handle)
	if err != nil {
		log.Error("check-pid-failed", err)
		return fmt.Errorf("stop: pid not found for container")
	}

	return c.stopper.StopAll(log, handle, []int{state.Pid}, kill)
}

// Destroy kills any container processes and deletes the bundle directory
func (c *Containerizer) Destroy(log lager.Logger, handle string) error {
	log = log.Session("destroy", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	state, err := c.runner.State(log, handle)
	if err != nil {
		log.Info("state-failed-skipping-kill", lager.Data{"error": err.Error()})
		return c.depot.Destroy(log, handle)
	}

	log.Info("state", lager.Data{
		"state": state,
	})

	if state.Status == runrunc.RunningStatus {
		if err := c.runner.Kill(log, handle); err != nil {
			log.Error("kill-failed", err)
			return err
		}
	}

	// wait for container to exit as a result of kill
	c.exits.Wait(handle)
	c.exits.Unstore(handle)

	return c.depot.Destroy(log, handle)
}

func (c *Containerizer) Info(log lager.Logger, handle string) (gardener.ActualContainerSpec, error) {
	bundlePath, err := c.depot.Lookup(log, handle)
	if err != nil {
		return gardener.ActualContainerSpec{}, err
	}

	return gardener.ActualContainerSpec{
		BundlePath: bundlePath,
		Events:     c.events.Events(handle),
	}, nil
}

func (c *Containerizer) Metrics(log lager.Logger, handle string) (gardener.ActualContainerMetrics, error) {
	return c.runner.Stats(log, handle)
}

func (c *Containerizer) CPULimit(log lager.Logger, handle string) (garden.CPULimits, error) {
	log = log.Session("containerizer-cpulimit", lager.Data{"handle": handle})

	bundlePath, err := c.depot.Lookup(log, handle)
	if err != nil {
		log.Error("looking-up-container", err)
		return garden.CPULimits{}, fmt.Errorf("looking up container: %s", err)
	}

	bndl, err := c.loader.Load(bundlePath)
	if err != nil {
		log.Error("loading-bundle", err)
		return garden.CPULimits{}, fmt.Errorf("loading bundle: %s", err)
	}

	return garden.CPULimits{
		LimitInShares: *(bndl.Resources().CPU.Shares),
	}, nil
}

// Handles returns a list of all container handles
func (c *Containerizer) Handles() ([]string, error) {
	return c.depot.Handles()
}
