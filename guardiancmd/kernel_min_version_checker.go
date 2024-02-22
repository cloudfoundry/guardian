package guardiancmd

import (
	"fmt"
	"regexp"
	"strconv"
)

var kernelVersionRegexp = regexp.MustCompile(`^(\d+)(?:\.(\d+))?(?:\.(\d+))?.*$`)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . SysctlGetter

type SysctlGetter interface {
	GetString(key string) (string, error)
}

type KernelMinVersionChecker struct {
	sysctlGetter SysctlGetter
}

func NewKernelMinVersionChecker(sysctlGetter SysctlGetter) KernelMinVersionChecker {
	return KernelMinVersionChecker{
		sysctlGetter: sysctlGetter,
	}
}

func (c KernelMinVersionChecker) CheckVersionIsAtLeast(maj, min, patch uint16) (bool, error) {
	minVersion := kernelVersion(maj, min, patch)

	kernelVersionStr, err := c.sysctlGetter.GetString("kernel.osrelease")
	if err != nil {
		return false, err
	}

	kernelVersion, err := kernelVersionFromReleaseString(kernelVersionStr)
	if err != nil {
		return false, err
	}

	return kernelVersion >= minVersion, nil
}

func kernelVersionFromReleaseString(release string) (uint64, error) {
	parts := kernelVersionRegexp.FindStringSubmatch(release)
	if len(parts) != 4 {
		return 0, fmt.Errorf("malformed version: %s", release)
	}

	maj, err := strconv.ParseInt(parts[1], 10, 16)
	if err != nil {
		return 0, err
	}

	var min int64
	if parts[2] != "" {
		min, err = strconv.ParseInt(parts[2], 10, 16)
		if err != nil {
			return 0, err
		}
	}

	var patch int64
	if parts[3] != "" {
		patch, err = strconv.ParseInt(parts[3], 10, 16)
		if err != nil {
			return 0, err
		}
	}

	return kernelVersion(uint16(maj), uint16(min), uint16(patch)), nil
}

func kernelVersion(maj, min, patch uint16) uint64 {
	return uint64(maj)<<32 + uint64(min)<<16 + uint64(patch)
}
