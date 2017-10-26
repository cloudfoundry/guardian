package quota

// Quota limit params - currently we only control blocks hard limit
type Quota struct {
	Size   uint64
	BCount uint64
}
