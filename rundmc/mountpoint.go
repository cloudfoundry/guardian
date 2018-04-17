package rundmc

import "github.com/docker/docker/pkg/mount"

//go:generate counterfeiter . MountPointChecker
type MountPointChecker func(path string) (bool, error)

//go:generate counterfeiter . MountOptionsGetter
type MountOptionsGetter func(path string, mountInfos []*mount.Info) ([]string, error)
