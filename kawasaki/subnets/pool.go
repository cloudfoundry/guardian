// The subnets package provides a subnet pool from which networks may be dynamically acquired or
// statically reserved.
package subnets

import (
	"fmt"
	"math"
	"net"
	"sync"

	"code.cloudfoundry.org/lager/v3"
)

//go:generate counterfeiter -o fake_subnet_pool/fake_pool.go . Pool
type Pool interface {
	// Allocates an IP address and associates it with a subnet. The subnet is selected by the given SubnetSelector.
	// The IP address is selected by the given IPSelector.
	// Returns a subnet, an IP address, and if either selector fails, an error is returned.
	Acquire(lager.Logger, SubnetSelector, IPSelector) (*net.IPNet, net.IP, error)

	// Releases an IP address associated with an allocated subnet. If the subnet has no other IP
	// addresses associated with it, it is deallocated.
	// Returns an error if the given combination is not already in the pool.
	Release(*net.IPNet, net.IP) error

	// Remove an IP address so it appears to be associated with the given subnet.
	Remove(*net.IPNet, net.IP) error

	// Returns the number of /30 subnets which can be Acquired by a DynamicSubnetSelector.
	Capacity() int

	// Run the provided callback if the given subnet is not in use
	RunIfFree(*net.IPNet, func() error) error
}

type pool struct {
	allocated    map[string][]net.IP // net.IPNet.String +> seq net.IP
	dynamicRange *net.IPNet
	mu           sync.Mutex
}

//go:generate counterfeiter . SubnetSelector

// SubnetSelector is a strategy for selecting a subnet.
type SubnetSelector interface {
	// Returns a subnet based on a dynamic range and some existing statically-allocated
	// subnets. If no suitable subnet can be found, returns an error.
	SelectSubnet(dynamic *net.IPNet, existing []*net.IPNet) (*net.IPNet, error)
}

//go:generate counterfeiter . IPSelector

// IPSelector is a strategy for selecting an IP address in a subnet.
type IPSelector interface {
	// Returns an IP address in the given subnet which is not one of the given existing
	// IP addresses. If no such IP address can be found, returns an error.
	SelectIP(subnet *net.IPNet, existing []net.IP) (net.IP, error)
}

func NewPool(ipNet *net.IPNet) Pool {
	return &pool{dynamicRange: ipNet, allocated: make(map[string][]net.IP)}
}

// Acquire uses the given subnet and IP selectors to request a subnet, container IP address combination
// from the pool.
func (p *pool) Acquire(log lager.Logger, sn SubnetSelector, i IPSelector) (subnet *net.IPNet, ip net.IP, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if subnet, err = sn.SelectSubnet(p.dynamicRange, existingSubnets(p.allocated)); err != nil {
		return nil, nil, err
	}

	ips := p.allocated[subnet.String()]
	existingIPs := append(ips, NetworkIP(subnet), GatewayIP(subnet), BroadcastIP(subnet))
	if ip, err = i.SelectIP(subnet, existingIPs); err != nil {
		return nil, nil, err
	}

	p.allocated[subnet.String()] = append(ips, ip)
	return subnet, ip, err
}

// Recover re-allocates a given subnet and ip address combination in the pool. It returns
// an error if the combination is already allocated.
func (p *pool) Remove(subnet *net.IPNet, ip net.IP) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ip == nil {
		return ErrIpCannotBeNil
	}

	for _, existing := range p.allocated[subnet.String()] {
		if existing.Equal(ip) {
			return ErrOverlapsExistingSubnet
		}
	}

	p.allocated[subnet.String()] = append(p.allocated[subnet.String()], ip)
	return nil
}

func (p *pool) Release(subnet *net.IPNet, ip net.IP) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	subnetString := subnet.String()
	ips := p.allocated[subnetString]

	if i, found := indexOf(ips, ip); found {
		if reducedIps, empty := removeIPAtIndex(ips, i); empty {
			delete(p.allocated, subnetString)
		} else {
			p.allocated[subnetString] = reducedIps
		}

		return nil
	}

	return ErrReleasedUnallocatedSubnet
}

// Capacity returns the number of /30 subnets that can be allocated
// from the pool's dynamic allocation range.
func (m *pool) Capacity() int {
	masked, total := m.dynamicRange.Mask.Size()
	return int(math.Pow(2, float64(total-masked)) / 4)
}

func (p *pool) RunIfFree(subnet *net.IPNet, cb func() error) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.allocated[subnet.String()]; ok {
		return nil
	}

	return cb()
}

// Returns the gateway IP of a given subnet, which is always the maximum valid IP
func GatewayIP(subnet *net.IPNet) net.IP {
	return next(subnet.IP)
}

// Returns the network IP of a subnet.
func NetworkIP(subnet *net.IPNet) net.IP {
	return subnet.IP
}

// Returns the broadcast IP of a subnet.
func BroadcastIP(subnet *net.IPNet) net.IP {
	return max(subnet)
}

// returns the keys in the given map whose values are non-empty slices
func existingSubnets(m map[string][]net.IP) (result []*net.IPNet) {
	for k, v := range m {
		if len(v) > 0 {
			_, ipn, err := net.ParseCIDR(k)
			if err != nil {
				panic(fmt.Sprintf("failed to parse a CIDR in the subnet pool: %s", err))
			}

			result = append(result, ipn)
		}
	}

	return result
}

func indexOf(a []net.IP, w net.IP) (int, bool) {
	for i, v := range a {
		if v.Equal(w) {
			return i, true
		}
	}

	return -1, false
}

// removeAtIndex removes from a slice at the given index,
// and returns the new slice and boolean, true iff the new slice is empty.
func removeIPAtIndex(ips []net.IP, i int) ([]net.IP, bool) {
	l := len(ips)
	ips[i] = ips[l-1]
	ips = ips[:l-1]
	return ips, l == 1
}
