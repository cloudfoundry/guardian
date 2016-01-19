package kawasaki

import (
	"fmt"
	"net"
	"strconv"

	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets"
	"github.com/pivotal-golang/lager"
)

const hostIntfKey = "kawasaki.host-interface"
const containerIntfKey = "kawasaki.container-interface"
const bridgeIntfKey = "kawasaki.bridge-interface"
const bridgeIpKey = "kawasaki.bridge-ip"
const containerIpKey = "kawasaki.container-ip"
const externalIpKey = "kawasaki.external-ip"
const subnetKey = "kawasaki.subnet"
const iptableChainKey = "kawasaki.iptable-chain"
const mtuKey = "kawasaki.mtu"

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
	Set(handle string, name string, value string)
	Get(handle string, name string) (string, error)
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
	FromPort     uint32
	ToPort       uint32
	IPTableChain string
	ContainerIP  net.IP
	ExternalIP   net.IP
}

type Networker struct {
	kawasakiBinPath string // path to a binary that will apply the configuration

	specParser    SpecParser
	subnetPool    subnets.Pool
	configCreator ConfigCreator
	configurer    Configurer
	configStore   ConfigStore
	portForwarder PortForwarder
	portPool      PortPool
	tag           string
}

func New(
	kawasakiBinPath string,
	specParser SpecParser,
	subnetPool subnets.Pool,
	configCreator ConfigCreator,
	configurer Configurer,
	configStore ConfigStore,
	portForwarder PortForwarder,
	portPool PortPool,
	tag string) *Networker {
	return &Networker{
		kawasakiBinPath: kawasakiBinPath,

		specParser:    specParser,
		subnetPool:    subnetPool,
		configCreator: configCreator,
		configurer:    configurer,
		configStore:   configStore,

		portForwarder: portForwarder,
		portPool:      portPool,
		tag:           tag,
	}
}

// Hook provides path and appropriate arguments to the kawasaki executable that
// applies the network configuration after the network namesapce creation.
func (n *Networker) Hook(log lager.Logger, handle, spec string) (gardener.Hook, error) {
	log = log.Session("network", lager.Data{
		"handle": handle,
		"spec":   spec,
	})

	log.Info("started")
	defer log.Info("finished")

	subnetReq, ipReq, err := n.specParser.Parse(log, spec)
	if err != nil {
		log.Error("parse-failed", err)
		return gardener.Hook{}, err
	}

	subnet, ip, err := n.subnetPool.Acquire(log, subnetReq, ipReq)
	if err != nil {
		log.Error("acquire-failed", err)
		return gardener.Hook{}, err
	}

	config, err := n.configCreator.Create(log, handle, subnet, ip)
	if err != nil {
		log.Error("create-config-failed", err)
		return gardener.Hook{}, fmt.Errorf("create network config: %s", err)
	}
	log.Info("config-create", lager.Data{"config": config})

	save(n.configStore, handle, config)

	return gardener.Hook{
		Path: n.kawasakiBinPath,
		Args: []string{
			n.kawasakiBinPath,
			fmt.Sprintf("--host-interface=%s", config.HostIntf),
			fmt.Sprintf("--container-interface=%s", config.ContainerIntf),
			fmt.Sprintf("--bridge-interface=%s", config.BridgeName),
			fmt.Sprintf("--bridge-ip=%s", config.BridgeIP),
			fmt.Sprintf("--container-ip=%s", config.ContainerIP),
			fmt.Sprintf("--external-ip=%s", config.ExternalIP),
			fmt.Sprintf("--subnet=%s", config.Subnet.String()),
			fmt.Sprintf("--iptable-chain=%s", config.IPTableChain),
			fmt.Sprintf("--mtu=%d", config.Mtu),
			fmt.Sprintf("--tag=%s", n.tag),
		},
	}, nil
}

// Capacity returns the number of subnets this network can host
func (n *Networker) Capacity() uint64 {
	return uint64(n.subnetPool.Capacity())
}

func (n *Networker) NetIn(handle string, externalPort, containerPort uint32) (uint32, uint32, error) {
	cfg, err := load(n.configStore, handle)
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
		FromPort:     externalPort,
		ToPort:       containerPort,
		IPTableChain: cfg.IPTableChain,
		ContainerIP:  cfg.ContainerIP,
		ExternalIP:   cfg.ExternalIP,
	})

	if err != nil {
		return 0, 0, err
	}

	return externalPort, containerPort, nil
}

func (n *Networker) Destroy(log lager.Logger, handle string) error {
	cfg, err := load(n.configStore, handle)
	if err != nil {
		return err
	}

	if err := n.configurer.Destroy(log, cfg); err != nil {
		log.Error("destroy-config-failed", err)
		return err
	}

	return n.subnetPool.Release(cfg.Subnet, cfg.ContainerIP)
}

func getAll(config ConfigStore, handle string, key ...string) (vals []string, err error) {
	for _, k := range key {
		v, err := config.Get(handle, k)
		if err != nil {
			return nil, err
		}

		vals = append(vals, v)
	}

	return vals, nil
}

func save(config ConfigStore, handle string, netConfig NetworkConfig) {
	config.Set(handle, hostIntfKey, netConfig.HostIntf)
	config.Set(handle, containerIntfKey, netConfig.ContainerIntf)
	config.Set(handle, bridgeIntfKey, netConfig.BridgeName)
	config.Set(handle, bridgeIpKey, netConfig.BridgeIP.String())
	config.Set(handle, containerIpKey, netConfig.ContainerIP.String())
	config.Set(handle, subnetKey, netConfig.Subnet.String())
	config.Set(handle, iptableChainKey, netConfig.IPTableChain)
	config.Set(handle, mtuKey, strconv.Itoa(netConfig.Mtu))
	config.Set(handle, externalIpKey, netConfig.ExternalIP.String())
}

func load(config ConfigStore, handle string) (NetworkConfig, error) {
	vals, err := getAll(config, handle, hostIntfKey, containerIntfKey, bridgeIntfKey, bridgeIpKey, containerIpKey, subnetKey, iptableChainKey, mtuKey, externalIpKey)

	if err != nil {
		return NetworkConfig{}, err
	}

	_, ipnet, err := net.ParseCIDR(vals[5])
	if err != nil {
		return NetworkConfig{}, err
	}

	mtu, err := strconv.Atoi(vals[7])
	if err != nil {
		return NetworkConfig{}, err
	}

	return NetworkConfig{
		HostIntf:      vals[0],
		ContainerIntf: vals[1],
		BridgeName:    vals[2],
		BridgeIP:      net.ParseIP(vals[3]),
		ContainerIP:   net.ParseIP(vals[4]),
		ExternalIP:    net.ParseIP(vals[8]),
		Subnet:        ipnet,
		IPTableChain:  vals[6],
		Mtu:           mtu,
	}, nil
}
