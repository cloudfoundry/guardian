package rundmc

// reverse of runc cgroups.ConvertCPUSharesToCgroupV2Value
func ConvertCgroupV2ValueToCPUShares(cpuWeight uint64) uint64 {
	if cpuWeight == 0 {
		return 0
	}
	return (cpuWeight-1)*262142/9999 + 2
}
