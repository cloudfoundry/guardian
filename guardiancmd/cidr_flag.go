package guardiancmd

import "net"

type CIDRFlag struct {
	cidr *net.IPNet
}

func (f *CIDRFlag) UnmarshalFlag(value string) error {
	_, ipNet, err := net.ParseCIDR(value)
	if err != nil {
		return err
	}

	f.cidr = ipNet

	return nil
}

func (f CIDRFlag) String() string {
	if f.cidr == nil {
		return ""
	}

	return f.cidr.String()
}

func (f CIDRFlag) CIDR() *net.IPNet {
	return f.cidr
}
