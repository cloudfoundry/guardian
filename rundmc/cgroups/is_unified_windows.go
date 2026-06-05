package cgroups

func IsCgroup2UnifiedMode() bool {
	return false
}

func ConvertCPUSharesToCgroupV2Value(cpuShares uint64) uint64 {
	return 0
}

func EnableSupportedControllers(_ string) error {
	return nil
}
