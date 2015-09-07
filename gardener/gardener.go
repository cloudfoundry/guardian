package gardener

import (
	"io"
	"time"

	"github.com/cloudfoundry-incubator/garden"
)

//go:generate counterfeiter . Containerizer
type Containerizer interface {
	Create(spec DesiredContainerSpec) error
}

type DesiredContainerSpec struct {
	Handle string
}

type Gardener struct {
	Containerizer Containerizer
}

func (g *Gardener) Create(spec garden.ContainerSpec) (garden.Container, error) {
	g.Containerizer.Create(DesiredContainerSpec{
		Handle: spec.Handle,
	})

	return &container{}, nil
}

func (g *Gardener) Start() error                                             { return nil }
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

func (g *Gardener) Lookup(handle string) (garden.Container, error) {
	return nil, nil
}

type container struct {
}

func (c *container) Handle() string {
	return ""
}

func (c *container) Run(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	return nil, nil
}

func (c *container) Stop(kill bool) error {
	return nil
}

func (c *container) Info() (garden.ContainerInfo, error) {
	return garden.ContainerInfo{}, nil
}

func (c *container) StreamIn(garden.StreamInSpec) error {
	return nil
}

func (c *container) StreamOut(garden.StreamOutSpec) (io.ReadCloser, error) {
	return nil, nil
}

func (c *container) LimitBandwidth(limits garden.BandwidthLimits) error {
	return nil
}

func (c *container) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	return garden.BandwidthLimits{}, nil
}

func (c *container) LimitCPU(limits garden.CPULimits) error {
	return nil
}

func (c *container) CurrentCPULimits() (garden.CPULimits, error) {
	return garden.CPULimits{}, nil
}

func (c *container) LimitDisk(limits garden.DiskLimits) error {
	return nil
}

func (c *container) CurrentDiskLimits() (garden.DiskLimits, error) {
	return garden.DiskLimits{}, nil
}

func (c *container) LimitMemory(limits garden.MemoryLimits) error {
	return nil
}

func (c *container) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	return garden.MemoryLimits{}, nil
}

func (c *container) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	return 0, 0, nil
}

func (c *container) NetOut(netOutRule garden.NetOutRule) error {
	return nil
}

func (c *container) Attach(processID uint32, io garden.ProcessIO) (garden.Process, error) {
	return nil, nil
}

func (c *container) Metrics() (garden.Metrics, error) {
	return garden.Metrics{}, nil
}

func (c *container) Properties() (garden.Properties, error) {
	return nil, nil
}

func (c *container) Property(name string) (string, error) {
	return "", nil
}

func (c *container) SetProperty(name string, value string) error {
	return nil
}

func (c *container) RemoveProperty(name string) error {
	return nil
}
