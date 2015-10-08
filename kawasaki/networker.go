package kawasaki

import (
	"fmt"
	"net"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets"
)

//go:generate counterfeiter . NetnsMgr
type NetnsMgr interface {
	Create(handle string) error
	Lookup(handle string) (string, error)
	Destroy(handle string) error
}

//go:generate counterfeiter . SpecParser
type SpecParser interface {
	Parse(spec string) (subnets.SubnetSelector, subnets.IPSelector, error)
}

//go:generate counterfeiter . ConfigCreator
type ConfigCreator interface {
	Create(handle string, subnet *net.IPNet, ip net.IP) (NetworkConfig, error)
}

//go:generate counterfeiter . ConfigApplier
type ConfigApplier interface {
	Apply(cfg NetworkConfig, nsPath string) error
}

type Networker struct {
	netnsMgr NetnsMgr

	specParser    SpecParser
	subnetPool    subnets.Pool
	configCreator ConfigCreator
	configApplier ConfigApplier
}

func New(netnsMgr NetnsMgr,
	specParser SpecParser,
	subnetPool subnets.Pool,
	configCreator ConfigCreator,
	configApplier ConfigApplier) *Networker {
	return &Networker{
		netnsMgr: netnsMgr,

		specParser:    specParser,
		subnetPool:    subnetPool,
		configCreator: configCreator,
		configApplier: configApplier,
	}
}

// Network configures a network namespace based on the given spec
// and returns the path to it
func (n *Networker) Network(handle, spec string) (string, error) {
	subnetReq, ipReq, err := n.specParser.Parse(spec)
	if err != nil {
		return "", err
	}

	subnet, ip, err := n.subnetPool.Acquire(subnetReq, ipReq)
	if err != nil {
		return "", err
	}

	config, err := n.configCreator.Create(handle, subnet, ip)
	if err != nil {
		return "", fmt.Errorf("create network config: %s", err)
	}

	err = n.netnsMgr.Create(handle)
	if err != nil {
		return "", err
	}

	path, err := n.netnsMgr.Lookup(handle)
	if err != nil {
		return "", err
	}

	if err := n.configApplier.Apply(config, path); err != nil {
		n.netnsMgr.Destroy(handle)
		return "", err
	}

	return path, nil
}
