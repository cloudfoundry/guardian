package fakedevices

import "net"
import "code.cloudfoundry.org/garden"

type FaveVethCreator struct {
	CreateCalledWith struct {
		HostIfcName, ContainerIfcName string
	}

	CreateReturns struct {
		Host, Container *net.Interface
		Err             error
	}
}

func (f *FaveVethCreator) Create(hostIfcName, containerIfcName string) (*net.Interface, *net.Interface, error) {
	f.CreateCalledWith.HostIfcName = hostIfcName
	f.CreateCalledWith.ContainerIfcName = containerIfcName

	return f.CreateReturns.Host, f.CreateReturns.Container, f.CreateReturns.Err
}

type InterfaceIPAndSubnet struct {
	Interface *net.Interface
	IP        net.IP
	Subnet    *net.IPNet
}

type FakeLink struct {
	AddIPCalledWith        []InterfaceIPAndSubnet
	SetUpCalledWith        []*net.Interface
	AddDefaultGWCalledWith struct {
		Interface *net.Interface
		IP        net.IP
	}

	SetMTUCalledWith struct {
		Interface *net.Interface
		MTU       int
	}

	SetNsCalledWith struct {
		Interface *net.Interface
		Fd        int
	}

	SetUpFunc           func(*net.Interface) error
	InterfaceByNameFunc func(string) (*net.Interface, bool, error)

	AddIPReturns        map[string]error
	AddDefaultGWReturns error
	SetMTUReturns       error
	SetNsReturns        error
	StatisticsReturns   error
}

func (f *FakeLink) AddIP(intf *net.Interface, ip net.IP, subnet *net.IPNet) error {
	f.AddIPCalledWith = append(f.AddIPCalledWith, InterfaceIPAndSubnet{intf, ip, subnet})
	return f.AddIPReturns[intf.Name]
}

func (f *FakeLink) AddDefaultGW(intf *net.Interface, ip net.IP) error {
	f.AddDefaultGWCalledWith.Interface = intf
	f.AddDefaultGWCalledWith.IP = ip
	return f.AddDefaultGWReturns
}

func (f *FakeLink) SetUp(intf *net.Interface) error {
	f.SetUpCalledWith = append(f.SetUpCalledWith, intf)
	if f.SetUpFunc == nil {
		return nil
	}

	return f.SetUpFunc(intf)
}

func (f *FakeLink) SetMTU(intf *net.Interface, mtu int) error {
	f.SetMTUCalledWith.Interface = intf
	f.SetMTUCalledWith.MTU = mtu
	return f.SetMTUReturns
}

func (f *FakeLink) SetNs(intf *net.Interface, fd int) error {
	f.SetNsCalledWith.Interface = intf
	f.SetNsCalledWith.Fd = fd
	return f.SetNsReturns
}

func (f *FakeLink) InterfaceByName(name string) (*net.Interface, bool, error) {
	if f.InterfaceByNameFunc != nil {
		return f.InterfaceByNameFunc(name)
	}

	return nil, false, nil
}

func (f *FakeLink) Statistics() (garden.ContainerNetworkStat, error) {
	if f.StatisticsReturns != nil {
		return garden.ContainerNetworkStat{}, f.StatisticsReturns
	}

	return garden.ContainerNetworkStat{
		RxBytes: 1,
		TxBytes: 2,
	}, nil
}

type FakeBridge struct {
	CreateCalledWith struct {
		Name   string
		IP     net.IP
		Subnet *net.IPNet
	}

	CreateReturns struct {
		Interface *net.Interface
		Error     error
	}

	AddCalledWith struct {
		Bridge, Slave *net.Interface
	}

	AddReturns error

	DestroyCalledWith []string

	DestroyReturns error
}

func (f *FakeBridge) Create(name string, ip net.IP, subnet *net.IPNet) (*net.Interface, error) {
	f.CreateCalledWith.Name = name
	f.CreateCalledWith.IP = ip
	f.CreateCalledWith.Subnet = subnet
	return f.CreateReturns.Interface, f.CreateReturns.Error
}

func (f *FakeBridge) Add(bridge, slave *net.Interface) error {
	f.AddCalledWith.Bridge = bridge
	f.AddCalledWith.Slave = slave
	return f.AddReturns
}

func (f *FakeBridge) Destroy(bridge string) error {
	f.DestroyCalledWith = append(f.DestroyCalledWith, bridge)
	return f.DestroyReturns
}
