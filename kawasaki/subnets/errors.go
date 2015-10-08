package subnets

import "errors"

var (
	// ErrInsufficientSubnets is returned by AcquireDynamically if no more subnets can be allocated.
	ErrInsufficientSubnets = errors.New("insufficient subnets remaining in the pool")

	// ErrInsufficientIPs is returned by AcquireDynamically if no more IPs can be allocated.
	ErrInsufficientIPs = errors.New("insufficient IPs remaining in the pool")

	// ErrReleasedUnallocatedNetwork is returned by Release if the subnet is not allocated.
	ErrReleasedUnallocatedSubnet = errors.New("subnet is not allocated")

	// ErrOverlapsExistingSubnet is returned if a recovered subnet overlaps an existing, allocated subnet
	ErrOverlapsExistingSubnet = errors.New("subnet overlaps an existing subnet")

	// ErrInvalidRange is returned by AcquireStatically and by Recover if the subnet range is invalid.
	ErrInvalidRange = errors.New("subnet has invalid range")

	// ErrInvalidIP is returned if a static IP is requested inside a subnet
	// which does not contain that IP
	ErrInvalidIP = errors.New("the requested IP is not within the subnet")

	// ErrIPAlreadyAcquired is returned if a static IP is requested which has already been allocated
	ErrIPAlreadyAcquired = errors.New("the requested IP is already allocated")

	// ErrIpCannotBeNil is returned by Release(..) and Recover(..) if a nil
	// IP address is passed.
	ErrIpCannotBeNil = errors.New("the IP field cannot be empty")

	ErrIPEqualsGateway   = errors.New("a container IP must not equal the gateway IP")
	ErrIPEqualsBroadcast = errors.New("a container IP must not equal the broadcast IP")
)
