package gardener

import (
	"time"

	"github.com/cloudfoundry-incubator/garden"
)

//go:generate counterfeiter . Containerizer
type Containerizer interface {
	Create(spec DesiredContainerSpec) error
	Run(handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Destroy(handle string) error
}

//go:generate counterfeiter . Networker
type Networker interface {
	Network(network string) (string, error)
}

//go:generate counterfeiter . UidGenerator
type UidGenerator interface {
	Generate() string
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
	// Containerizer runs and manages linux containers
	Containerizer Containerizer

	// UidGenerator generates unique ids for containers
	UidGenerator UidGenerator

	// Starter runs any needed start-up tasks (e.g. setting up cgroups)
	Starter

	// Networker creates a network for containers
	Networker Networker
}

func (g *Gardener) Create(spec garden.ContainerSpec) (garden.Container, error) {
	if spec.Handle == "" {
		spec.Handle = g.UidGenerator.Generate()
	}

	networkPath, err := g.Networker.Network(spec.Network)
	if err != nil {
		return nil, err
	}

	if err := g.Containerizer.Create(DesiredContainerSpec{
		Handle:      spec.Handle,
		NetworkPath: networkPath,
	}); err != nil {
		return nil, err
	}

	return g.Lookup(spec.Handle)
}

func (g *Gardener) Lookup(handle string) (garden.Container, error) {
	return &container{
		handle:        handle,
		containerizer: g.Containerizer,
	}, nil
}

func (g *Gardener) Destroy(handle string) error {
	return g.Containerizer.Destroy(handle)
}

func (g *Gardener) Stop()                                                    {}
func (g *Gardener) GraceTime(garden.Container) time.Duration                 { return 0 }
func (g *Gardener) Ping() error                                              { return nil }
func (g *Gardener) Capacity() (garden.Capacity, error)                       { return garden.Capacity{}, nil }
func (g *Gardener) Containers(garden.Properties) ([]garden.Container, error) { return nil, nil }

func (g *Gardener) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	return nil, nil
}

func (g *Gardener) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	return nil, nil
}
