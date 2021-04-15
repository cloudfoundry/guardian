package bundlerules

import (
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
	limit := int64(spec.Limits.Memory.LimitInBytes)

	var swapLimit *int64
	if !l.DisableSwapLimit {
		swapLimit = &limit
	}

	bndl = bndl.WithMemoryLimit(specs.LinuxMemory{Limit: &limit, Swap: swapLimit})

	shares := uint64(spec.Limits.CPU.LimitInShares)
	if spec.Limits.CPU.Weight > 0 {
		shares = uint64(spec.Limits.CPU.Weight)
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

	pids := int64(spec.Limits.Pid.Max)
	return bndl.WithPidLimit(specs.LinuxPids{Limit: pids}), nil
}

func int64PtrVal(n uint64) *int64 {
	unsignedVal := int64(n)
	return &unsignedVal
}
