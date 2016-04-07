package gardener

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . SysInfoProvider
//go:generate counterfeiter . Containerizer
//go:generate counterfeiter . Networker
//go:generate counterfeiter . VolumeCreator
//go:generate counterfeiter . UidGenerator

const ContainerIPKey = "garden.network.container-ip"
const BridgeIPKey = "garden.network.host-ip"
const ExternalIPKey = "garden.network.external-ip"
const MappedPortsKey = "garden.network.mapped-ports"

type SysInfoProvider interface {
	TotalMemory() (uint64, error)
	TotalDisk() (uint64, error)
}

type Containerizer interface {
	Create(log lager.Logger, spec DesiredContainerSpec) error
	StreamIn(log lager.Logger, handle string, spec garden.StreamInSpec) error
	StreamOut(log lager.Logger, handle string, spec garden.StreamOutSpec) (io.ReadCloser, error)
	Run(log lager.Logger, handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Destroy(log lager.Logger, handle string) error
	Info(log lager.Logger, handle string) (ActualContainerSpec, error)
	Metrics(log lager.Logger, handle string) (ActualContainerMetrics, error)
	CPULimit(log lager.Logger, handle string) (garden.CPULimits, error)
	Handles() ([]string, error)
}

type Networker interface {
	Hooks(log lager.Logger, handle, spec string) (Hooks, error)
	Capacity() uint64
	Destroy(log lager.Logger, handle string) error
	NetIn(log lager.Logger, handle string, hostPort, containerPort uint32) (uint32, uint32, error)
	NetOut(log lager.Logger, handle string, rule garden.NetOutRule) error
}

type VolumeCreator interface {
	Create(log lager.Logger, handle string, spec rootfs_provider.Spec) (string, []string, error)
	Destroy(log lager.Logger, handle string) error
	Metrics(log lager.Logger, handle string) (garden.ContainerDiskStat, error)
	GC(log lager.Logger) error
}

type UidGenerator interface {
	Generate() string
}

//go:generate counterfeiter . PropertyManager

type PropertyManager interface {
	All(handle string) (props garden.Properties, err error)
	Set(handle string, name string, value string)
	Remove(handle string, name string) error
	Get(handle string, name string) (string, error)
	MatchesAll(handle string, props garden.Properties) bool
	DestroyKeySpace(string) error
}

type Starter interface {
	Start() error
}

type UidGeneratorFunc func() string

func (fn UidGeneratorFunc) Generate() string {
	return fn()
}

type Hooks struct {
	Prestart Hook
	Poststop Hook
}

type Hook struct {
	Path string
	Args []string
}

type DesiredContainerSpec struct {
	Handle string

	// Path to the Root Filesystem for the container
	RootFSPath string

	// Network hook
	NetworkHooks Hooks

	// Bind mounts
	BindMounts []garden.BindMount

	// Container is privileged
	Privileged bool

	Limits garden.Limits

	Env []string
}

type ActualContainerSpec struct {
	// The path to the container's bundle directory
	BundlePath string

	// Whether the container is stopped
	Stopped bool

	// Process IDs (not PIDs) of processes in the container
	ProcessIDs []string

	// Events (e.g. OOM) which have occured in the container
	Events []string
}

type ActualContainerMetrics struct {
	CPU    garden.ContainerCPUStat
	Memory garden.ContainerMemoryStat
}

// Gardener orchestrates other components to implement the Garden API
type Gardener struct {
	// SysInfoProvider returns total memory and total disk
	SysInfoProvider SysInfoProvider

	// Containerizer runs and manages linux containers
	Containerizer Containerizer

	// UidGenerator generates unique ids for containers
	UidGenerator UidGenerator

	// Starter runs any needed start-up tasks (e.g. setting up cgroups)
	Starter

	// Networker creates a network for containers
	Networker Networker

	// VolumeCreator creates volumes for containers
	VolumeCreator VolumeCreator

	Logger lager.Logger

	// PropertyManager creates map of container properties
	PropertyManager PropertyManager

	// MaxContainers limits the advertised container capacity
	MaxContainers uint64
}

// Create creates a container by combining the results of networker.Network,
// volumizer.Create and containzer.Create.
func (g *Gardener) Create(spec garden.ContainerSpec) (ctr garden.Container, err error) {
	if err := g.checkDuplicateHandle(spec.Handle); err != nil {
		return nil, err
	}

	if spec.Handle == "" {
		spec.Handle = g.UidGenerator.Generate()
	}

	log := g.Logger.Session("create", lager.Data{"handle": spec.Handle})

	log.Info("start")
	defer log.Info("created")

	defer func() {
		if err != nil {
			log := log.Session("cleanup")
			log.Info("start")
			g.destroy(log, spec.Handle)
			log.Info("cleanedup")
		}
	}()

	hooks, err := g.Networker.Hooks(log, spec.Handle, spec.Network)
	if err != nil {
		return nil, err
	}

	rootFSURL, err := url.Parse(spec.RootFSPath)
	if err != nil {
		return nil, err
	}

	if err := g.VolumeCreator.GC(log); err != nil {
		log.Error("graph-cleanup-failed", err)
	}

	rootFSPath, env, err := g.VolumeCreator.Create(log, spec.Handle, rootfs_provider.Spec{
		RootFS:     rootFSURL,
		QuotaSize:  int64(spec.Limits.Disk.ByteHard),
		QuotaScope: spec.Limits.Disk.Scope,
		Namespaced: !spec.Privileged,
	})
	if err != nil {
		return nil, err
	}

	if err := g.Containerizer.Create(log, DesiredContainerSpec{
		Handle:       spec.Handle,
		RootFSPath:   rootFSPath,
		NetworkHooks: hooks,
		Privileged:   spec.Privileged,
		BindMounts:   spec.BindMounts,
		Limits:       spec.Limits,
		Env:          append(env, spec.Env...),
	}); err != nil {
		return nil, err
	}

	container, err := g.Lookup(spec.Handle)
	if err != nil {
		return nil, err
	}

	for name, value := range spec.Properties {
		err := container.SetProperty(name, value)
		if err != nil {
			return nil, err
		}
	}

	return container, nil
}

func (g *Gardener) Lookup(handle string) (garden.Container, error) {
	return g.lookup(handle), nil
}

func (g *Gardener) lookup(handle string) garden.Container {
	return &container{
		logger:          g.Logger,
		handle:          handle,
		containerizer:   g.Containerizer,
		volumeCreator:   g.VolumeCreator,
		networker:       g.Networker,
		propertyManager: g.PropertyManager,
	}
}

// Destroy idempotently destroys any resources associated with the given handle
func (g *Gardener) Destroy(handle string) error {
	log := g.Logger.Session("destroy", lager.Data{"handle": handle})

	log.Info("start")
	defer log.Info("destroyed")

	return g.destroy(log, handle)
}

func (g *Gardener) destroy(log lager.Logger, handle string) error {
	if err := g.Containerizer.Destroy(g.Logger, handle); err != nil {
		return err
	}

	if err := g.Networker.Destroy(g.Logger, handle); err != nil {
		return err
	}

	if err := g.VolumeCreator.Destroy(g.Logger, handle); err != nil {
		return err
	}

	return g.PropertyManager.DestroyKeySpace(handle)
}

func (g *Gardener) Stop()                                    {}
func (g *Gardener) GraceTime(garden.Container) time.Duration { return 0 }
func (g *Gardener) Ping() error                              { return nil }

func (g *Gardener) Capacity() (garden.Capacity, error) {
	mem, err := g.SysInfoProvider.TotalMemory()
	if err != nil {
		return garden.Capacity{}, err
	}

	disk, err := g.SysInfoProvider.TotalDisk()
	if err != nil {
		return garden.Capacity{}, err
	}

	cap := g.Networker.Capacity()
	if g.MaxContainers > 0 && g.MaxContainers < cap {
		cap = g.MaxContainers
	}

	return garden.Capacity{
		MemoryInBytes: mem,
		DiskInBytes:   disk,
		MaxContainers: cap,
	}, nil
}

func (g *Gardener) Containers(props garden.Properties) ([]garden.Container, error) {
	log := g.Logger.Session("list-containers")

	log.Info("starting")
	defer log.Info("finished")

	handles, err := g.Containerizer.Handles()
	if err != nil {
		log.Error("handles-failed", err)
		return []garden.Container{}, err
	}

	var containers []garden.Container
	for _, handle := range handles {
		if g.PropertyManager.MatchesAll(handle, props) {
			containers = append(containers, g.lookup(handle))
		}
	}

	return containers, nil
}

func (g *Gardener) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	result := make(map[string]garden.ContainerInfoEntry)
	for _, handle := range handles {
		container := g.lookup(handle)

		var infoErr *garden.Error = nil
		info, err := container.Info()
		if err != nil {
			infoErr = garden.NewError(err.Error())
		}
		result[handle] = garden.ContainerInfoEntry{
			Info: info,
			Err:  infoErr,
		}
	}

	return result, nil
}

func (g *Gardener) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	result := make(map[string]garden.ContainerMetricsEntry)
	for _, handle := range handles {
		var e *garden.Error
		m, err := g.lookup(handle).Metrics()
		if err != nil {
			e = garden.NewError(err.Error())
		}

		result[handle] = garden.ContainerMetricsEntry{
			Err:     e,
			Metrics: m,
		}
	}

	return result, nil
}

func (g *Gardener) checkDuplicateHandle(handle string) error {
	handles, err := g.Containerizer.Handles()
	if err != nil {
		return err
	}
	for _, h := range handles {
		if h == handle {
			return errors.New(fmt.Sprintf("Handle '%s' already in use", handle))
		}
	}
	return nil
}
