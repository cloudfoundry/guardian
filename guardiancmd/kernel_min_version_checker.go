package guardiancmd

import (
	"fmt"

	"github.com/hashicorp/go-version"
)

//go:generate counterfeiter . SysctlGetter

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

func (c KernelMinVersionChecker) CheckVersionIsAtLeast(maj, min, patch int) (bool, error) {
	minVersion := version.Must(version.NewVersion(fmt.Sprintf("%d.%d.%d", maj, min, patch)))

	kernelVersion, err := c.sysctlGetter.GetString("kernel.osrelease")
	if err != nil {
		return false, err
	}

	kernelSemver, err := version.NewVersion(kernelVersion)
	if err != nil {
		return false, err
	}

	return kernelSemver.Core().GreaterThanOrEqual(minVersion), nil
}
