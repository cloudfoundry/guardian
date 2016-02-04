package bundlerules

import "os"

//go:generate counterfeiter . MkdirChowner

type MkdirChowner interface {
	MkdirChown(path string, perms os.FileMode, uid, gid int) error
}

type MkdirChownFunc func(path string, perms os.FileMode, uid, gid int) error

func (fn MkdirChownFunc) MkdirChown(path string, perms os.FileMode, uid, gid int) error {
	return fn(path, perms, uid, gid)
}

func MkdirChown(path string, perms os.FileMode, uid, gid int) error {
	if err := os.MkdirAll(path, perms); err != nil {
		return err
	}

	return os.Chown(path, uid, gid)
}
