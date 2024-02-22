package throttle

//counterfeiter:generate . MemoryProvider
type MemoryProvider interface {
	TotalMemory() (uint64, error)
}
