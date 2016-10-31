package locksmith

type Unlocker interface {
	Unlock() error
}
