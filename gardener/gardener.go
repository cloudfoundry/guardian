package gardener

import (
	"time"

	"github.com/cloudfoundry-incubator/garden"
)

//go:generate counterfeiter . Containerizer
type Containerizer interface {
	Create(spec DesiredContainerSpec) error
	Run(handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
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
}

type Gardener struct {
	Containerizer Containerizer
	UidGenerator  UidGenerator

	Starter
}

func (g *Gardener) Create(spec garden.ContainerSpec) (garden.Container, error) {
	if spec.Handle == "" {
		spec.Handle = g.UidGenerator.Generate()
	}

	if err := g.Containerizer.Create(DesiredContainerSpec{
		Handle: spec.Handle,
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

func (g *Gardener) Stop()                                                    {}
func (g *Gardener) GraceTime(garden.Container) time.Duration                 { return 0 }
func (g *Gardener) Ping() error                                              { return nil }
func (g *Gardener) Capacity() (garden.Capacity, error)                       { return garden.Capacity{}, nil }
func (g *Gardener) Destroy(handle string) error                              { return nil }
func (g *Gardener) Containers(garden.Properties) ([]garden.Container, error) { return nil, nil }

func (g *Gardener) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	return nil, nil
}

func (g *Gardener) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	return nil, nil
}
