package groot

import "path"

//go:generate counterfeiter . Bundle
type Bundle interface {
	Path() string
	RootFsPath() string
}

type bundle struct {
	path string
}

func NewBundle(path string) *bundle {
	return &bundle{
		path: path,
	}
}

func (bdl *bundle) Path() string {
	return bdl.path
}

func (bdl *bundle) RootFsPath() string {
	return path.Join(bdl.path, "rootfs")
}
