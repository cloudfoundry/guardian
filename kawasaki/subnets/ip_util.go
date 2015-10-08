package subnets

import "net"

func equals(a *net.IPNet, b *net.IPNet) bool {
	aOnes, aBits := a.Mask.Size()
	bOnes, bBits := b.Mask.Size()
	return a.IP.Equal(b.IP) && (aOnes == bOnes) && (aBits == bBits)
}

func overlaps(a *net.IPNet, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}

func next(ip net.IP) net.IP {
	next := clone(ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			return next
		}
	}

	panic("overflowed maximum IP")
}

func clone(ip net.IP) net.IP {
	clone := make([]byte, len(ip))
	copy(clone, ip)
	return clone
}

func max(ipn *net.IPNet) net.IP {
	mask := ipn.Mask
	min := clone(ipn.IP)

	if len(mask) != len(min) {
		panic("length of mask is not compatible with length of network IP")
	}

	max := make([]byte, len(min))
	for i, b := range mask {
		max[i] = min[i] | ^b
	}

	return net.IP(max).To16()
}
