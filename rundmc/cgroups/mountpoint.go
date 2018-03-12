package cgroups

//go:generate counterfeiter . MountPointChecker

type MountPointChecker func(path string) (bool, error)
