package bundlerules

import (
	"math"
	"os"
	"strconv"
	"strings"

	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
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
	IOMaxReadBps     uint64
	IOMaxWriteBps    uint64
	IOMaxReadIOPS    uint64
	IOMaxWriteIOPS   uint64
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

	blockIO := specs.LinuxBlockIO{Weight: &l.BlockIOWeight}

	if cgroups.IsCgroup2UnifiedMode() && (l.IOMaxReadBps > 0 || l.IOMaxWriteBps > 0 || l.IOMaxReadIOPS > 0 || l.IOMaxWriteIOPS > 0) {
		devices := getAllBlockDevices()

		var readBps, writeBps, readIOPS, writeIOPS []specs.LinuxThrottleDevice
		for _, dev := range devices {
			if l.IOMaxReadBps > 0 {
				readBps = append(readBps, specs.LinuxThrottleDevice{LinuxBlockIODevice: dev, Rate: l.IOMaxReadBps})
			}
			if l.IOMaxWriteBps > 0 {
				writeBps = append(writeBps, specs.LinuxThrottleDevice{LinuxBlockIODevice: dev, Rate: l.IOMaxWriteBps})
			}
			if l.IOMaxReadIOPS > 0 {
				readIOPS = append(readIOPS, specs.LinuxThrottleDevice{LinuxBlockIODevice: dev, Rate: l.IOMaxReadIOPS})
			}
			if l.IOMaxWriteIOPS > 0 {
				writeIOPS = append(writeIOPS, specs.LinuxThrottleDevice{LinuxBlockIODevice: dev, Rate: l.IOMaxWriteIOPS})
			}
		}
		blockIO.ThrottleReadBpsDevice = readBps
		blockIO.ThrottleWriteBpsDevice = writeBps
		blockIO.ThrottleReadIOPSDevice = readIOPS
		blockIO.ThrottleWriteIOPSDevice = writeIOPS
	}

	bndl = bndl.WithBlockIO(blockIO)

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

// getAllBlockDevices reads /sys/block and returns major:minor for all real
// block devices (excludes loop, ram, and dm virtual devices).
func getAllBlockDevices() []specs.LinuxBlockIODevice {
	var devices []specs.LinuxBlockIODevice
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return devices
	}
	for _, e := range entries {
		name := e.Name()
		// Skip virtual devices
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "dm-") {
			continue
		}
		data, err := os.ReadFile("/sys/block/" + name + "/dev")
		if err != nil {
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(string(data)), ":", 2)
		if len(parts) != 2 {
			continue
		}
		major, errMaj := strconv.ParseInt(parts[0], 10, 64)
		minor, errMin := strconv.ParseInt(parts[1], 10, 64)
		if errMaj != nil || errMin != nil {
			continue
		}
		if major == 0 && minor == 0 {
			continue
		}
		devices = append(devices, specs.LinuxBlockIODevice{Major: major, Minor: minor})
	}
	return devices
}
