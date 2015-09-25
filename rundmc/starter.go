package rundmc

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"

	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/pivotal-golang/lager"
)

type Starter struct {
	*CgroupStarter
}

func NewStarter(procCgroupReader io.ReadCloser, cgroupMountpoint string, runner command_runner.CommandRunner) *Starter {
	return &Starter{
		&CgroupStarter{
			CgroupPath:    cgroupMountpoint,
			ProcCgroups:   procCgroupReader,
			CommandRunner: runner,
		},
	}
}

type CgroupStarter struct {
	CgroupPath    string
	CommandRunner command_runner.CommandRunner

	ProcCgroups io.ReadCloser
}

func (s *CgroupStarter) Start() error {
	return s.mountCgroupsIfNeeded()
}

func (s *CgroupStarter) mountCgroupsIfNeeded() error {
	defer s.ProcCgroups.Close()
	if err := os.MkdirAll(s.CgroupPath, 0755); err != nil {
		return err
	}

	if !s.isMountPoint(s.CgroupPath) {
		s.mountTmpfsOnCgroupPath(s.CgroupPath)
	}

	scanner := bufio.NewScanner(s.ProcCgroups)

	scanner.Scan()
	scanner.Scan() // ignore header

	for scanner.Scan() {
		var cgroupInProcCgroups string
		if n, err := fmt.Sscanf(scanner.Text(), "%s ", &cgroupInProcCgroups); err != nil || n != 1 {
			continue
		}

		if err := s.mountCgroup(path.Join(s.CgroupPath, cgroupInProcCgroups), cgroupInProcCgroups); err != nil {
			return err
		}
	}

	return nil
}

func (s *CgroupStarter) mountTmpfsOnCgroupPath(path string) {
	s.CommandRunner.Run(exec.Command("mount", "-t", "tmpfs", "-o", "uid=0,gid=0,mode=0755", "cgroup", path))
}

func (s *CgroupStarter) mountCgroup(cgroupPath, cgroupType string) error {
	mlog := plog.Start("setup-cgroup")

	if !s.isMountPoint(cgroupPath) {
		if err := os.MkdirAll(cgroupPath, 0755); err != nil {
			return mlog.Err("mkdir", err)
		}

		cmd := exec.Command("mount", "-n", "-t", "cgroup", "-o", cgroupType, "cgroup", cgroupPath)
		cmd.Stderr = mlog.Start("mount-cgroup", lager.Data{"path": cgroupPath})
		if err := s.CommandRunner.Run(cmd); err != nil {
			return mlog.Err("mount-cgroup", err)
		}
	}

	mlog.Info("setup-cgroup-complete")
	return nil
}

func (s *CgroupStarter) isMountPoint(path string) bool {
	return s.CommandRunner.Run(exec.Command("mountpoint", "-q", path)) == nil
}
