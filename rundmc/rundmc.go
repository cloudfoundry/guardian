package rundmc

import (
	"io"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gardener"
)

//go:generate counterfeiter . Depot
type Depot interface {
	Create(handle string) error
	Lookup(handle string) (path string, err error)
}

//go:generate counterfeiter . Checker
type Checker interface {
	Check(stdout, stderr io.Reader) error
}

//go:generate counterfeiter . ContainerRunner
type ContainerRunner interface {
	Start(path string, io garden.ProcessIO) (garden.Process, error)
	Exec(path string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
}

type Containerizer struct {
	Depot           Depot
	ContainerRunner ContainerRunner
	StartCheck      Checker
}

func (c *Containerizer) Create(spec gardener.DesiredContainerSpec) error {
	if err := c.Depot.Create(spec.Handle); err != nil {
		return err
	}

	path, err := c.Depot.Lookup(spec.Handle)
	if err != nil {
		return err
	}

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	c.ContainerRunner.Start(path, garden.ProcessIO{Stdout: stdoutW, Stderr: stderrW})

	if err := c.StartCheck.Check(stdoutR, stderrR); err != nil {
		return err
	}

	return nil
}

func (c *Containerizer) Run(handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	path, err := c.Depot.Lookup(handle)
	if err != nil {
		return nil, err
	}

	return c.ContainerRunner.Exec(path, spec, io)
}
