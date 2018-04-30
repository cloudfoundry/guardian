package cgrouper

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func GetCGroupPath(cgroupsRoot, subsystem, tag string, privileged bool) (string, error) {
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

	return filepath.Join(cgroupsRoot, subsystem, currentCgroup, parentCgroup), nil
}

func CleanGardenCgroups(cGroupsPath, tag string) error {
	subsystems, err := ioutil.ReadDir(cGroupsPath)
	if err != nil {
		return err
	}

	for _, subsystem := range subsystems {
		cGroupPath, err := GetCGroup(subsystem.Name())
		if err != nil {
			return err
		}
		nestedCgroup := filepath.Join(cGroupsPath, subsystem.Name(), cGroupPath)
		path := filepath.Join(nestedCgroup, "garden-"+tag)
		rmErr := os.Remove(path)
		if rmErr != nil && !os.IsNotExist(rmErr) {
			procInfo, infoErr := getCgroupProcInfos(path)
			if infoErr != nil {
				return fmt.Errorf("%s: %s", rmErr, infoErr)
			}
			return fmt.Errorf("%s: %s", rmErr, procInfo)
		}
	}

	return nil
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

func getCgroupProcInfos(cgroupFs string) (string, error) {
	pids, readErr := ioutil.ReadFile(filepath.Join(cgroupFs, "cgroup.procs"))
	if readErr != nil {
		return "", fmt.Errorf("error reading cgroup.procs: %s", readErr)
	}
	var procInfos []string
	for _, pid := range strings.Split(strings.TrimSpace(string(pids)), "\n") {
		procInfo, infoErr := ioutil.ReadFile(filepath.Join("/proc", pid, "cmdline"))
		if infoErr != nil {
			return "", fmt.Errorf("getting proc info: %s", infoErr)
		}
		procInfos = append(procInfos, string(procInfo))
	}
	return strings.Join(procInfos, "\n"), nil
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
