package store // import "code.cloudfoundry.org/grootfs/store"

import "path"

type Bundle struct {
	path string
}

func NewBundle(path string) *Bundle {
	return &Bundle{
		path: path,
	}
}

func (bdl *Bundle) Path() string {
	return bdl.path
}

func (bdl *Bundle) RootFSPath() string {
	return path.Join(bdl.path, "rootfs")
}
