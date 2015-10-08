package subnets

import (
	"fmt"
	"net"
)

// StaticSubnetSelector requests a specific ("static") subnet. Returns an error if the subnet is already allocated.
type StaticSubnetSelector struct {
	*net.IPNet
}

func (s StaticSubnetSelector) SelectSubnet(dynamic *net.IPNet, existing []*net.IPNet) (*net.IPNet, error) {
	if overlaps(dynamic, s.IPNet) {
		return nil, fmt.Errorf("the requested subnet (%v) overlaps the dynamic allocation range (%v)", s.IPNet.String(), dynamic.String())
	}

	for _, e := range existing {
		if overlaps(s.IPNet, e) && !equals(s.IPNet, e) {
			return nil, fmt.Errorf("the requested subnet (%v) overlaps an existing subnet (%v)", s.IPNet.String(), e.String())
		}
	}

	return s.IPNet, nil
}

type dynamicSubnetSelector int

// DynamicSubnetSelector requests the next unallocated ("dynamic") subnet from the dynamic range.
// Returns an error if there are no remaining subnets in the dynamic range.
var DynamicSubnetSelector dynamicSubnetSelector = 0

func (dynamicSubnetSelector) SelectSubnet(dynamic *net.IPNet, existing []*net.IPNet) (*net.IPNet, error) {
	exists := make(map[string]bool)
	for _, e := range existing {
		exists[e.String()] = true
	}

	min := dynamic.IP
	mask := net.CIDRMask(30, 32) // /30
	for ip := min; dynamic.Contains(ip); ip = next(ip) {
		subnet := &net.IPNet{IP: ip, Mask: mask}
		ip = next(next(next(ip)))
		if dynamic.Contains(ip) && !exists[subnet.String()] {
			return subnet, nil
		}
	}

	return nil, ErrInsufficientSubnets
}

// StaticIPSelector requests a specific ("static") IP address. Returns an error if the IP is already
// allocated, or if it is outside the given subnet.
type StaticIPSelector struct {
	net.IP
}

func (s StaticIPSelector) SelectIP(subnet *net.IPNet, existing []net.IP) (net.IP, error) {
	if BroadcastIP(subnet).Equal(s.IP) {
		return nil, ErrIPEqualsBroadcast
	}

	if GatewayIP(subnet).Equal(s.IP) {
		return nil, ErrIPEqualsGateway
	}

	if !subnet.Contains(s.IP) {
		return nil, ErrInvalidIP
	}

	for _, e := range existing {
		if e.Equal(s.IP) {
			return nil, ErrIPAlreadyAcquired
		}
	}

	return s.IP, nil
}

type dynamicIPSelector int

// DynamicIPSelector requests the next available ("dynamic") IP address from a given subnet.
// Returns an error if no more IP addresses remain in the subnet.
var DynamicIPSelector dynamicIPSelector = 0

func (dynamicIPSelector) SelectIP(subnet *net.IPNet, existing []net.IP) (net.IP, error) {
	exists := make(map[string]bool)
	for _, e := range existing {
		exists[e.String()] = true
	}

	for i := subnet.IP; subnet.Contains(i); i = next(i) {
		if !exists[i.String()] {
			return i, nil
		}
	}

	return nil, ErrInsufficientIPs
}
