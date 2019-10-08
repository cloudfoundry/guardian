package mount

type NilUnmounter struct {
}

func (u NilUnmounter) Unmount(path string) error {
	panic("not-implemented-with-nil-unmounter")
}
