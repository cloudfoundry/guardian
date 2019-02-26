package cpuentitlement

import "code.cloudfoundry.org/guardian/gardener"

type Calculator struct {
	SysInfoProvider gardener.SysInfoProvider
}

func (c Calculator) CalculateDefaultEntitlementPerShare() (float64, error) {
	cpuCores, err := c.SysInfoProvider.CPUCores()
	if err != nil {
		return 0, err
	}

	memory, err := c.SysInfoProvider.TotalMemory()
	if err != nil {
		return 0, err
	}

	return float64(cpuCores*100) / float64(memory/1024/1024), nil
}
