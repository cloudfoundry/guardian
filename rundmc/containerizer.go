package rundmc

import (
	"fmt"
	"io"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Depot
//go:generate counterfeiter . OCIRuntime
//go:generate counterfeiter . NstarRunner
//go:generate counterfeiter . EventStore
//go:generate counterfeiter . BundleLoader
//go:generate counterfeiter . Stopper
//go:generate counterfeiter . StateStore
//go:generate counterfeiter . RootfsFileCreator

type Depot interface {
	Create(log lager.Logger, handle string, spec gardener.DesiredContainerSpec) error
	Lookup(log lager.Logger, handle string) (path string, err error)
	Destroy(log lager.Logger, handle string) error
	Handles() ([]string, error)
}

type BundleLoader interface {
	Load(path string) (goci.Bndl, error)
}

type OCIRuntime interface {
	Create(log lager.Logger, bundlePath, id string, io garden.ProcessIO) error
	Exec(log lager.Logger, bundlePath, id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Attach(log lager.Logger, bundlePath, id, processId string, io garden.ProcessIO) (garden.Process, error)
	Kill(log lager.Logger, bundlePath string) error
	Delete(log lager.Logger, bundlePath string) error
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

type StateStore interface {
	StoreStopped(handle string)
	IsStopped(handle string) bool
}

type RootfsFileCreator interface {
	CreateFiles(rootFSPath string, pathsToCreate ...string) error
}

// Containerizer knows how to manage a depot of container bundles
type Containerizer struct {
	depot             Depot
	loader            BundleLoader
	runtime           OCIRuntime
	stopper           Stopper
	nstar             NstarRunner
	events            EventStore
	states            StateStore
	rootfsFileCreator RootfsFileCreator
}

func New(depot Depot, runtime OCIRuntime, loader BundleLoader, nstarRunner NstarRunner, stopper Stopper, events EventStore, states StateStore, rootfsFileCreator RootfsFileCreator) *Containerizer {
	return &Containerizer{
		depot:             depot,
		runtime:           runtime,
		loader:            loader,
		nstar:             nstarRunner,
		stopper:           stopper,
		events:            events,
		states:            states,
		rootfsFileCreator: rootfsFileCreator,
	}
}

// Create creates a bundle in the depot and starts its init process
func (c *Containerizer) Create(log lager.Logger, spec gardener.DesiredContainerSpec) error {
	log = log.Session("containerizer-create", lager.Data{"handle": spec.Handle})

	log.Info("start")
	defer log.Info("finished")

	if err := c.rootfsFileCreator.CreateFiles(spec.RootFSPath, "/etc/hosts", "/etc/resolv.conf"); err != nil {
		log.Error("create-rootfs-mountpoint-files-failed", err)
		return err
	}

	if err := c.depot.Create(log, spec.Handle, spec); err != nil {
		log.Error("depot-create-failed", err)
		return err
	}

	path, err := c.depot.Lookup(log, spec.Handle)
	if err != nil {
		log.Error("lookup-failed", err)
		return err
	}

	if err = c.runtime.Create(log, path, spec.Handle, garden.ProcessIO{}); err != nil {
		log.Error("runtime-create-failed", err)
		return err
	}

	go func() {
		if err := c.runtime.WatchEvents(log, spec.Handle, c.events); err != nil {
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
		log.Error("lookup-failed", err)
		return nil, err
	}

	return c.runtime.Exec(log, path, handle, spec, io)
}

func (c *Containerizer) Attach(log lager.Logger, handle string, processID string, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("attach", lager.Data{"handle": handle, "process-id": processID})

	log.Info("started")
	defer log.Info("finished")

	path, err := c.depot.Lookup(log, handle)
	if err != nil {
		log.Error("lookup-failed", err)
		return nil, err
	}

	return c.runtime.Attach(log, path, handle, processID, io)
}

// StreamIn streams files in to the container
func (c *Containerizer) StreamIn(log lager.Logger, handle string, spec garden.StreamInSpec) error {
	log = log.Session("stream-in", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	state, err := c.runtime.State(log, handle)
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

	state, err := c.runtime.State(log, handle)
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

	state, err := c.runtime.State(log, handle)
	if err != nil {
		log.Error("check-pid-failed", err)
		return fmt.Errorf("stop: pid not found for container: %s", err)
	}

	if err = c.stopper.StopAll(log, handle, []int{state.Pid}, kill); err != nil {
		log.Error("stop-all-failed", err, lager.Data{"pid": state.Pid})
		return fmt.Errorf("stop: %s", err)
	}

	c.states.StoreStopped(handle)
	return nil
}

// Destroy deletes the container and the bundle directory
func (c *Containerizer) Destroy(log lager.Logger, handle string) error {
	log = log.Session("destroy", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	state, err := c.runtime.State(log, handle)
	if err != nil {
		log.Info("state-failed-skipping-delete", lager.Data{"error": err.Error()})
		return nil
	}

	log.Info("state", lager.Data{
		"state": state,
	})

	if state.Status == runrunc.CreatedStatus || state.Status == runrunc.StoppedStatus {
		if err := c.runtime.Delete(log, handle); err != nil {
			log.Error("delete-failed", err)
			return err
		}
	}

	return nil
}

func (c *Containerizer) RemoveBundle(log lager.Logger, handle string) error {
	log = log.Session("depot", lager.Data{"handle": handle})
	return c.depot.Destroy(log, handle)
}

func (c *Containerizer) Info(log lager.Logger, handle string) (gardener.ActualContainerSpec, error) {
	bundlePath, err := c.depot.Lookup(log, handle)
	if err != nil {
		return gardener.ActualContainerSpec{}, err
	}

	bundle, err := c.loader.Load(bundlePath)
	if err != nil {
		return gardener.ActualContainerSpec{}, err
	}

	state, err := c.runtime.State(log, handle)
	if err != nil {
		return gardener.ActualContainerSpec{}, err
	}

	privileged := true
	for _, ns := range bundle.Namespaces() {
		if ns.Type == specs.UserNamespace {
			privileged = false
			break
		}
	}

	return gardener.ActualContainerSpec{
		Pid:        state.Pid,
		BundlePath: bundlePath,
		RootFSPath: bundle.RootFS(),
		Events:     c.events.Events(handle),
		Stopped:    c.states.IsStopped(handle),
		Limits: garden.Limits{
			CPU: garden.CPULimits{
				LimitInShares: *bundle.Resources().CPU.Shares,
			},
			Memory: garden.MemoryLimits{
				LimitInBytes: *bundle.Resources().Memory.Limit,
			},
		},
		Privileged: privileged,
	}, nil
}

func (c *Containerizer) Metrics(log lager.Logger, handle string) (gardener.ActualContainerMetrics, error) {
	return c.runtime.Stats(log, handle)
}

// Handles returns a list of all container handles
func (c *Containerizer) Handles() ([]string, error) {
	return c.depot.Handles()
}
