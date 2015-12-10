package kawasaki

import (
	"errors"
	"fmt"
	"net"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . NetnsMgr

type NetnsMgr interface {
	Create(log lager.Logger, handle string) error
	Lookup(log lager.Logger, handle string) (string, error)
	Destroy(log lager.Logger, handle string) error
}

//go:generate counterfeiter . SpecParser

type SpecParser interface {
	Parse(log lager.Logger, spec string) (subnets.SubnetSelector, subnets.IPSelector, error)
}

//go:generate counterfeiter . ConfigCreator

type ConfigCreator interface {
	Create(log lager.Logger, handle string, subnet *net.IPNet, ip net.IP) (NetworkConfig, error)
}

//go:generate counterfeiter . Configurer

type Configurer interface {
	Apply(log lager.Logger, cfg NetworkConfig, nsPath string) error
	Destroy(log lager.Logger, cfg NetworkConfig) error
}

//go:generate counterfeiter . ConfigStore

type ConfigStore interface {
	Put(handle string, cfg NetworkConfig)
	Get(handle string) (NetworkConfig, error)
	Remove(handle string)
}

type ConfigMap map[string]NetworkConfig

func (m ConfigMap) Put(handle string, cfg NetworkConfig) {
	m[handle] = cfg
}

func (m ConfigMap) Get(handle string) (NetworkConfig, error) {
	v, ok := m[handle]
	if !ok {
		return NetworkConfig{}, errors.New("Handle does not exist")
	}

	return v, nil
}

func (m ConfigMap) Remove(handle string) {
	delete(m, handle)
}

//go:generate counterfeiter . PortPool

type PortPool interface {
	Acquire() (uint32, error)
}

//go:generate counterfeiter . PortForwarder

type PortForwarder interface {
	Forward(spec *PortForwarderSpec) error
}

type PortForwarderSpec struct {
	NetworkConfig *NetworkConfig
	FromPort      uint32
	ToPort        uint32
}

type Networker struct {
	netnsMgr NetnsMgr

	specParser    SpecParser
	subnetPool    subnets.Pool
	configCreator ConfigCreator
	configurer    Configurer
	configStore   ConfigStore
	portForwarder PortForwarder
	portPool      PortPool
}

func New(netnsMgr NetnsMgr,
	specParser SpecParser,
	subnetPool subnets.Pool,
	configCreator ConfigCreator,
	configurer Configurer,
	configStore ConfigStore,
	portForwarder PortForwarder,
	portPool PortPool) *Networker {
	return &Networker{
		netnsMgr: netnsMgr,

		specParser:    specParser,
		subnetPool:    subnetPool,
		configCreator: configCreator,
		configurer:    configurer,
		configStore:   configStore,

		portForwarder: portForwarder,
		portPool:      portPool,
	}
}

// Network configures a network namespace based on the given spec
// and returns the path to it
func (n *Networker) Network(log lager.Logger, handle, spec string) (string, error) {
	log = log.Session("network", lager.Data{
		"handle": handle,
		"spec":   spec,
	})

	log.Info("started")
	defer log.Info("finished")

	subnetReq, ipReq, err := n.specParser.Parse(log, spec)
	if err != nil {
		log.Error("parse-failed", err)
		return "", err
	}

	subnet, ip, err := n.subnetPool.Acquire(log, subnetReq, ipReq)
	if err != nil {
		log.Error("acquire-failed", err)
		return "", err
	}

	config, err := n.configCreator.Create(log, handle, subnet, ip)
	if err != nil {
		log.Error("create-config-failed", err)
		return "", fmt.Errorf("create network config: %s", err)
	}

	n.configStore.Put(handle, config)

	err = n.netnsMgr.Create(log, handle)
	if err != nil {
		log.Error("create-netns-failed", err)
		return "", err
	}

	path, err := n.netnsMgr.Lookup(log, handle)
	if err != nil {
		log.Error("lookup-netns-failed", err)
		return "", err
	}

	if err := n.configurer.Apply(log, config, path); err != nil {
		log.Error("apply-config-failed", err)
		n.destroyOrLog(log, handle)
		return "", err
	}

	return path, nil
}

// Capacity returns the number of subnets this network can host
func (n *Networker) Capacity() uint64 {
	return uint64(n.subnetPool.Capacity())
}

func (n *Networker) NetIn(handle string, externalPort, containerPort uint32) (uint32, uint32, error) {
	netConfig, err := n.configStore.Get(handle)
	if err != nil {
		return 0, 0, err
	}

	if externalPort == 0 {
		externalPort, err = n.portPool.Acquire()
		if err != nil {
			return 0, 0, err
		}
	}

	if containerPort == 0 {
		containerPort = externalPort
	}

	err = n.portForwarder.Forward(&PortForwarderSpec{
		FromPort:      externalPort,
		ToPort:        containerPort,
		NetworkConfig: &netConfig,
	})

	if err != nil {
		return 0, 0, err
	}

	return externalPort, containerPort, nil
}

func (n *Networker) Destroy(log lager.Logger, handle string) error {
	cfg, err := n.configStore.Get(handle)
	if err != nil {
		return err
	}
	n.configStore.Remove(handle)

	if err := n.netnsMgr.Destroy(log, handle); err != nil {
		log.Error("destroy-namespace-failed", err)
		return err
	}

	if err := n.configurer.Destroy(log, cfg); err != nil {
		log.Error("destroy-config-failed", err)
		return err
	}

	return nil
}

func (n *Networker) destroyOrLog(log lager.Logger, handle string) {
	if err := n.Destroy(log, handle); err != nil {
		log.Error("destroy-failed", err)
	}
}
