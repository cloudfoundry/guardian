package cgrouper

import (
	"io/ioutil"
	"strings"

	"golang.org/x/sys/unix"
)

func UnmountCgroups(cgroupsRoot string) error {
	mountsFileContent, err := ioutil.ReadFile("/proc/self/mounts")
	if err != nil {
		return err
	}

	mountInfos := strings.Split(string(mountsFileContent), "\n")
	for _, info := range mountInfos {
		if info == "" {
			continue
		}

		fields := strings.Fields(info)
		if fields[2] == "cgroup" && strings.Contains(fields[1], cgroupsRoot) {
			if err := unix.Unmount(fields[1], unix.MNT_DETACH); err != nil {
				return err
			}
		}
	}

	return unix.Unmount(cgroupsRoot, 0)
}
