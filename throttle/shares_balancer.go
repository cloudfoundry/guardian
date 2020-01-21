package throttle

//go:generate counterfeiter . MemoryProvider
type MemoryProvider interface {
	TotalMemory() (uint64, error)
}
