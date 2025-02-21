package guardiancmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
)

type DeviceCgroupRuleFlag specs.LinuxDeviceCgroup

func (f *DeviceCgroupRuleFlag) UnmarshalFlag(value string) error {
	arr := strings.Split(value, " ")
	if len(arr) != 3 {
		return fmt.Errorf("invalid device specification: %s", value)
	}
	f.Type = strings.TrimSpace(arr[0])

	versions := strings.Split(arr[1], ":")
	if len(versions) != 2 {
		return fmt.Errorf("invalid device versions: %s", value)
	}

	var err error
	f.Major, err = parseDeviceVersion(versions[0])
	if err != nil {
		return fmt.Errorf("invalid device major version: %s", value)
	}
	f.Minor, err = parseDeviceVersion(versions[1])
	if err != nil {
		return fmt.Errorf("invalid device minor version: %s", value)
	}

	f.Access = strings.TrimSpace(arr[2])
	f.Allow = true

	return nil
}

func parseDeviceVersion(value string) (*int64, error) {
	var v *int64
	if value == "*" {
		v = intRef(-1)
	} else {
		vi, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, err
		}
		v = intRef(vi)
	}
	return v, nil
}

func (f DeviceCgroupRuleFlag) LinuxDeviceCgroup() specs.LinuxDeviceCgroup {
	return specs.LinuxDeviceCgroup(f)
}

func findMatchingCgroup(cgroupRule specs.LinuxDeviceCgroup, cgroupRules []DeviceCgroupRuleFlag) bool {
	for _, r := range cgroupRules {
		if cgroupRule.Type == r.Type {
			if *r.Major == -1 && *r.Minor == -1 {
				return true
			}
			if *cgroupRule.Major == *r.Major &&
				*cgroupRule.Minor == *r.Minor {
				return true
			}
		}
	}
	return false
}
