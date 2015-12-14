package gardener

import (
	"net/url"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . SysInfoProvider

type SysInfoProvider interface {
	TotalMemory() (uint64, error)
	TotalDisk() (uint64, error)
}

//go:generate counterfeiter . Containerizer

type Containerizer interface {
	Create(log lager.Logger, spec DesiredContainerSpec) error
	Run(log lager.Logger, handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Destroy(log lager.Logger, handle string) error
	Handles() ([]string, error)
}

//go:generate counterfeiter . Networker

type Networker interface {
	Network(log lager.Logger, handle, spec string) (string, error)
	Capacity() uint64
}

//go:generate counterfeiter . VolumeCreator

type VolumeCreator interface {
	Create(handle string, spec rootfs_provider.Spec) (string, []string, error)
}

//go:generate counterfeiter . UidGenerator

type UidGenerator interface {
	Generate() string
}

//go:generate counterfeiter . PropertyManager

type PropertyManager interface {
	Properties() (garden.Properties, error)
	SetProperty(name string, value string) error
	RemoveProperty(string) error
	Property(string) (string, error)
}

type Starter interface {
	Start() error
}

type UidGeneratorFunc func() string

func (fn UidGeneratorFunc) Generate() string {
	return fn()
}

type DesiredContainerSpec struct {
	Handle string

	// Path to the Root Filesystem for the container
	RootFSPath string

	// Path to a Network Namespace to enter
	NetworkPath string
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
}

func (g *Gardener) Create(spec garden.ContainerSpec) (garden.Container, error) {
	log := g.Logger.Session("create")

	if spec.Handle == "" {
		spec.Handle = g.UidGenerator.Generate()
	}

	networkPath, err := g.Networker.Network(log, spec.Handle, spec.Network)
	if err != nil {
		return nil, err
	}

	rootFSURL, err := url.Parse(spec.RootFSPath)
	if err != nil {
		return nil, err
	}

	rootFSPath, _, err := g.VolumeCreator.Create(spec.Handle, rootfs_provider.Spec{RootFS: rootFSURL})
	if err != nil {
		return nil, err
	}

	if err := g.Containerizer.Create(log, DesiredContainerSpec{
		Handle:      spec.Handle,
		RootFSPath:  rootFSPath,
		NetworkPath: networkPath,
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
			panic(err)
		}
	}

	return container, nil
}

func (g *Gardener) Lookup(handle string) (garden.Container, error) {
	return &container{
		handle:          handle,
		containerizer:   g.Containerizer,
		logger:          g.Logger,
		propertyManager: g.PropertyManager,
	}, nil
}

func (g *Gardener) Destroy(handle string) error {
	return g.Containerizer.Destroy(g.Logger, handle)
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

	return garden.Capacity{
		MemoryInBytes: mem,
		DiskInBytes:   disk,
		MaxContainers: cap,
	}, nil
}

func (g *Gardener) Containers(garden.Properties) ([]garden.Container, error) {
	containers := []garden.Container{}

	handles, err := g.Containerizer.Handles()
	if err != nil {
		return containers, err
	}

	for _, handle := range handles {
		container, err := g.Lookup(handle)
		if err != nil {
			return []garden.Container{}, err
		}
		containers = append(containers, container)
	}

	return containers, nil
}

func (g *Gardener) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	return nil, nil
}

func (g *Gardener) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	return nil, nil
}
