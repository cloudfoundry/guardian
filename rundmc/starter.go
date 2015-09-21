package rundmc

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"

	"github.com/cloudfoundry/gunk/command_runner"
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
	defer s.ProcCgroups.Close()
	if err := os.MkdirAll(s.CgroupPath, 0755); err != nil {
		return err
	}

	if err := s.CommandRunner.Run(exec.Command("mountpoint", "-q", s.CgroupPath)); err != nil {
		s.CommandRunner.Run(exec.Command("mount", "-t", "tmpfs", "-o", "uid=0,gid=0,mode=0755", "cgroup", s.CgroupPath))
	}

	scanner := bufio.NewScanner(s.ProcCgroups)

	scanner.Scan()
	scanner.Scan() // ignore header

	for scanner.Scan() {
		var cgroup string
		if n, err := fmt.Sscanf(scanner.Text(), "%s ", &cgroup); err != nil || n != 1 {
			continue
		}

		if err := s.CommandRunner.Run(exec.Command("mountpoint", "-q", path.Join(s.CgroupPath, cgroup))); err != nil {
			if err := os.MkdirAll(path.Join(s.CgroupPath, cgroup), 0755); err != nil {
				return err
			}

			s.CommandRunner.Run(exec.Command("mount", "-n", "-t", "cgroup", "-o", cgroup, "cgroup", path.Join(s.CgroupPath, cgroup)))
		}
	}

	return nil
}
