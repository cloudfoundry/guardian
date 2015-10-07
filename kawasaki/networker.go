package kawasaki

import "fmt"

//go:generate counterfeiter . NetnsMgr
type NetnsMgr interface {
	Create(handle string) error
	Lookup(handle string) (string, error)
	Destroy(handle string) error
}

//go:generate counterfeiter . ConfigCreator
type ConfigCreator interface {
	Create(handle, spec string) (NetworkConfig, error)
}

//go:generate counterfeiter . ConfigApplier
type ConfigApplier interface {
	Apply(cfg NetworkConfig, nsPath string) error
}

type Networker struct {
	netnsMgr NetnsMgr

	configCreator ConfigCreator
	configApplier ConfigApplier
}

func New(netnsMgr NetnsMgr,
	configCreator ConfigCreator,
	configApplier ConfigApplier) *Networker {
	return &Networker{
		netnsMgr:      netnsMgr,
		configCreator: configCreator,
		configApplier: configApplier,
	}
}

// Network configures a network namespace based on the given spec
// and returns the path to it
func (n *Networker) Network(handle, spec string) (string, error) {
	config, err := n.configCreator.Create(handle, spec)
	if err != nil {
		return "", fmt.Errorf("create network config: %s", err)
	}

	if err := n.netnsMgr.Create(handle); err != nil {
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
