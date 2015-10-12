package rundmc

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"

	"github.com/cloudfoundry-incubator/guardian/logging"
	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/pivotal-golang/lager"
)

type Starter struct {
	*CgroupStarter
}

func NewStarter(logger lager.Logger, procCgroupReader io.ReadCloser, cgroupMountpoint string, runner command_runner.CommandRunner) *Starter {
	return &Starter{
		&CgroupStarter{
			CgroupPath:    cgroupMountpoint,
			ProcCgroups:   procCgroupReader,
			CommandRunner: runner,
			Logger:        logger,
		},
	}
}

type CgroupStarter struct {
	CgroupPath    string
	CommandRunner command_runner.CommandRunner

	ProcCgroups io.ReadCloser
	Logger      lager.Logger
}

func (s *CgroupStarter) Start() error {
	return s.mountCgroupsIfNeeded(s.Logger)
}

func (s *CgroupStarter) mountCgroupsIfNeeded(log lager.Logger) error {
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

		if err := s.mountCgroup(log, path.Join(s.CgroupPath, cgroupInProcCgroups), cgroupInProcCgroups); err != nil {
			return err
		}
	}

	return nil
}

func (s *CgroupStarter) mountTmpfsOnCgroupPath(path string) {
	s.CommandRunner.Run(exec.Command("mount", "-t", "tmpfs", "-o", "uid=0,gid=0,mode=0755", "cgroup", path))
}

func (s *CgroupStarter) mountCgroup(log lager.Logger, cgroupPath, cgroupType string) error {
	log = log.Session("setup-cgroup", lager.Data{
		"path": cgroupPath,
		"type": cgroupType,
	})

	log.Info("started")
	defer log.Info("finished")

	if !s.isMountPoint(cgroupPath) {
		if err := os.MkdirAll(cgroupPath, 0755); err != nil {
			log.Error("mkdir-failed", err)
			return err
		}

		cmd := exec.Command("mount", "-n", "-t", "cgroup", "-o", cgroupType, "cgroup", cgroupPath)
		cmd.Stderr = logging.Writer(log.Session("mount-cgroup-cmd"))
		if err := s.CommandRunner.Run(cmd); err != nil {
			log.Error("mount-cgroup-failed", err)
			return err
		}
	}

	return nil
}

func (s *CgroupStarter) isMountPoint(path string) bool {
	return s.CommandRunner.Run(exec.Command("mountpoint", "-q", path)) == nil
}
