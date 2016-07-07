package kawasaki

import (
	"math"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/pivotal-golang/lager"
)

type CompositeNetworker struct {
	Networkers []Networker
}

func (c *CompositeNetworker) Capacity() (m uint64) {
	m = math.MaxUint64
	for _, networker := range c.Networkers {
		m = min(networker.Capacity(), m)
	}

	return m
}

func (c *CompositeNetworker) Destroy(log lager.Logger, handle string) error {
	for _, networker := range c.Networkers {
		if err := networker.Destroy(log, handle); err != nil {
			return err
		}
	}
	return nil
}

func (c *CompositeNetworker) NetIn(log lager.Logger, handle string, externalPort, containerPort uint32) (uint32, uint32, error) {
	return c.Networkers[0].NetIn(log, handle, externalPort, containerPort)
}

func (c *CompositeNetworker) NetOut(log lager.Logger, handle string, rule garden.NetOutRule) error {
	return c.Networkers[0].NetOut(log, handle, rule)
}

func (c *CompositeNetworker) Restore(log lager.Logger, handle string) error {
	for _, networker := range c.Networkers {
		if err := networker.Restore(log, handle); err != nil {
			return err
		}
	}
	return nil
}

func (c *CompositeNetworker) Network(log lager.Logger, containerSpec garden.ContainerSpec, pid int, bundlePath string) error {
	for _, networker := range c.Networkers {
		if err := networker.Network(log, containerSpec, pid, bundlePath); err != nil {
			return err
		}
	}
	return nil
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}

	return b
}
