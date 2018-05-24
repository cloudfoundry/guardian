package cgrouper

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

func GetCGroupPath(cgroupsRootPath, subsystem, tag string, privileged bool) (string, error) {
	parentCgroup := "garden"
	if tag != "" {
		parentCgroup = fmt.Sprintf("garden-%s", tag)
	}

	// We always use the cgroup root for privileged containers, regardless of
	// tag.
	if privileged {
		parentCgroup = ""
	}

	currentCgroup, err := GetCGroup(subsystem)
	if err != nil {
		return "", err
	}

	return filepath.Join(cgroupsRootPath, subsystem, currentCgroup, parentCgroup), nil
}

// GetCGroup, when running inside a container, returns the relative path of
// the cgroup in the host.
// E.g. /6d8612e9-cf2c-48d7-669e-249a546683f7, where 6d8612e9-cf2c-48d7-669e-249a546683f7
// is the container id.
func GetCGroup(subsystemToFind string) (string, error) {
	cgroupContent, err := ioutil.ReadFile(fmt.Sprintf("/proc/self/cgroup"))
	if err != nil {
		return "", err
	}

	cgroups := strings.Split(string(cgroupContent), "\n")
	for _, cgroup := range cgroups {
		fields := strings.Split(cgroup, ":")
		if len(fields) != 3 {
			return "", errors.New("Error parsing cgroups:" + cgroup)
		}
		subsystems := strings.Split(fields[1], ",")
		if containsElement(subsystems, subsystemToHierarchyID(subsystemToFind)) {
			return fields[2], nil
		}
	}
	return "", errors.New("Cgroup subsystem not found: " + subsystemToFind)
}

func containsElement(strings []string, elem string) bool {
	for _, e := range strings {
		if e == elem {
			return true
		}
	}
	return false
}

func subsystemToHierarchyID(subsystem string) string {
	if subsystem == "systemd" {
		// On Xenial there is a dedicated systemd named cgroup hirarchy (created by systemd itself) that looks like this in /proc/self/cgroup:
		// 1:name=systemd:/user.slice/user-1000.slice/session-3.scope
		// Here we "translate" the systemd "subsystem" name to hierarchy id so that it can be located in /proc/self/cgroup
		// Do note that the systemd cgroup named hierarchy is not available on Trusty though
		return "name=systemd"
	}
	return subsystem
}
