package bundlerules

import (
	"math"

	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var (
	CpuPeriod   uint64 = 100000
	MinCpuQuota uint64 = 1000
)

type Limits struct {
	CpuQuotaPerShare uint64
	BlockIOWeight    uint16
	DisableSwapLimit bool
}

func (l Limits) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec) (goci.Bndl, error) {
	var limit int64
	if spec.Limits.Memory.LimitInBytes > math.MaxInt64 {
		limit = math.MaxInt64
	} else {
		// #nosec G115 - any values over maxint64 are capped above, so no overflow
		limit = int64(spec.Limits.Memory.LimitInBytes)
	}

	var swapLimit *int64
	if !l.DisableSwapLimit {
		swapLimit = &limit
	}

	bndl = bndl.WithMemoryLimit(specs.LinuxMemory{Limit: &limit, Swap: swapLimit})

	//lint:ignore SA1019 - we still specify this to make the deprecated logic work until we get rid of the code in garden
	shares := spec.Limits.CPU.LimitInShares
	if spec.Limits.CPU.Weight > 0 {
		shares = spec.Limits.CPU.Weight
	}
	cpuSpec := specs.LinuxCPU{Shares: &shares}
	if l.CpuQuotaPerShare > 0 && shares > 0 {
		cpuSpec.Period = &CpuPeriod

		quota := shares * l.CpuQuotaPerShare
		if quota < MinCpuQuota {
			quota = MinCpuQuota
		}
		cpuSpec.Quota = int64PtrVal(quota)
	}
	bndl = bndl.WithCPUShares(cpuSpec)

	bndl = bndl.WithBlockIO(specs.LinuxBlockIO{Weight: &l.BlockIOWeight})

	var pids int64
	if spec.Limits.Pid.Max > math.MaxInt64 {
		pids = math.MaxInt64
	} else {
		// #nosec G115 - any values over maxint64 are capped above, so no overflow
		pids = int64(spec.Limits.Pid.Max)
	}

	// runc-specs now use a pointer to an int, and treat 0 as a valid limit. Previously this was "no limit"
	// so convert a 0 value to nil, since we will always want at least one process in our containers
	var pidPtr *int64
	if pids != 0 {
		pidPtr = &pids
	}
	return bndl.WithPidLimit(specs.LinuxPids{Limit: pidPtr}), nil
}

func int64PtrVal(n uint64) *int64 {
	if n > math.MaxInt64 {
		n = math.MaxInt64
	}
	// #nosec G115 - any values over maxint64 are capped above, so no overflow
	unsignedVal := int64(n)
	return &unsignedVal
}
