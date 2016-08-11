package store

import "path"

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

func (bdl *bundle) RootFSPath() string {
	return path.Join(bdl.path, "rootfs")
}
