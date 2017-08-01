package cgroups

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/lager"
)

type Starter struct {
	*CgroupStarter
}

const cgroupsHeader = "#subsys_name hierarchy num_cgroups enabled"

type CgroupsFormatError struct {
	Content string
}

func (err CgroupsFormatError) Error() string {
	return fmt.Sprintf("unknown /proc/cgroups format: %s", err.Content)
}

func NewStarter(logger lager.Logger, procCgroupReader io.ReadCloser, procSelfCgroupReader io.ReadCloser, cgroupMountpoint, gardenCgroup string, runner commandrunner.CommandRunner, chowner Chowner) *Starter {
	return &Starter{
		&CgroupStarter{
			CgroupPath:      cgroupMountpoint,
			GardenCgroup:    gardenCgroup,
			ProcCgroups:     procCgroupReader,
			ProcSelfCgroups: procSelfCgroupReader,
			CommandRunner:   runner,
			Logger:          logger,
			Chowner:         chowner,
		},
	}
}

type CgroupStarter struct {
	CgroupPath    string
	GardenCgroup  string
	CommandRunner commandrunner.CommandRunner

	ProcCgroups     io.ReadCloser
	ProcSelfCgroups io.ReadCloser

	Logger  lager.Logger
	Chowner Chowner
}

func (s *CgroupStarter) Start() error {
	return s.mountCgroupsIfNeeded(s.Logger)
}

func (s *CgroupStarter) mountCgroupsIfNeeded(logger lager.Logger) error {
	defer s.ProcCgroups.Close()
	defer s.ProcSelfCgroups.Close()
	if err := os.MkdirAll(s.CgroupPath, 0755); err != nil {
		return err
	}

	if !s.isMountPoint(s.CgroupPath) {
		s.mountTmpfsOnCgroupPath(logger, s.CgroupPath)
	} else {
		logger.Info("cgroups-tmpfs-already-mounted", lager.Data{"path": s.CgroupPath})
	}

	subsystemGroupings, err := s.subsystemGroupings()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(s.ProcCgroups)

	if !scanner.Scan() {
		return CgroupsFormatError{Content: "(empty)"}
	}

	if _, err := fmt.Sscanf(scanner.Text(), cgroupsHeader); err != nil {
		return CgroupsFormatError{Content: scanner.Text()}
	}

	for scanner.Scan() {
		var subsystem string
		var skip, enabled int
		n, err := fmt.Sscanf(scanner.Text(), "%s %d %d %d ", &subsystem, &skip, &skip, &enabled)
		if err != nil || n != 4 {
			return CgroupsFormatError{Content: scanner.Text()}
		}

		if enabled == 0 {
			continue
		}

		subsystemToMount, dirToCreate := subsystem, s.GardenCgroup
		if v, ok := subsystemGroupings[subsystem]; ok {
			subsystemToMount = v.SubSystem
			dirToCreate = path.Join(v.Path, s.GardenCgroup)
		}

		subsystemMountPath := path.Join(s.CgroupPath, subsystem)
		if err := s.idempotentCgroupMount(logger, subsystemMountPath, subsystemToMount); err != nil {
			return err
		}

		gardenCgroupPath := path.Join(s.CgroupPath, subsystem, dirToCreate)
		if err := s.createGardenCgroup(logger, gardenCgroupPath); err != nil {
			return err
		}
	}

	return nil
}

func (s *CgroupStarter) createGardenCgroup(log lager.Logger, gardenCgroupPath string) error {
	log = log.Session("creating-garden-cgroup", lager.Data{"gardenCgroup": gardenCgroupPath})
	log.Info("started")
	defer log.Info("finished")

	if err := os.MkdirAll(gardenCgroupPath, 0700); err != nil {
		return err
	}

	if err := s.Chowner.RecursiveChown(gardenCgroupPath); err != nil {
		return err
	}

	return nil
}

func (s *CgroupStarter) mountTmpfsOnCgroupPath(log lager.Logger, path string) {
	log = log.Session("cgroups-tmpfs-mounting", lager.Data{"path": path})
	log.Info("started")

	if err := s.CommandRunner.Run(exec.Command("mount", "-t", "tmpfs", "-o", "uid=0,gid=0,mode=0755", "cgroup", path)); err != nil {
		log.Error("mount-failed-continuing-anyway", err)
	} else {
		log.Info("finished")
	}
}

type group struct {
	SubSystem string
	Path      string
}

func (s *CgroupStarter) subsystemGroupings() (map[string]group, error) {
	groupings := map[string]group{}

	scanner := bufio.NewScanner(s.ProcSelfCgroups)
	for scanner.Scan() {
		segs := strings.Split(scanner.Text(), ":")
		if len(segs) != 3 {
			continue
		}

		subsystems := strings.Split(segs[1], ",")
		for _, subsystem := range subsystems {
			groupings[subsystem] = group{segs[1], segs[2]}
		}
	}

	return groupings, scanner.Err()
}

func (s *CgroupStarter) idempotentCgroupMount(logger lager.Logger, cgroupPath, subsystems string) error {
	logger = logger.Session("mount-cgroup", lager.Data{
		"path":       cgroupPath,
		"subsystems": subsystems,
	})

	logger.Info("started")

	if !s.isMountPoint(cgroupPath) {
		if err := os.MkdirAll(cgroupPath, 0755); err != nil {
			return fmt.Errorf("mkdir '%s': %s", cgroupPath, err)
		}

		cmd := exec.Command("mount", "-n", "-t", "cgroup", "-o", subsystems, "cgroup", cgroupPath)
		cmd.Stderr = logging.Writer(logger.Session("mount-cgroup-cmd"))
		if err := s.CommandRunner.Run(cmd); err != nil {
			return fmt.Errorf("mounting subsystems '%s' in '%s': %s", subsystems, cgroupPath, err)
		}
	} else {
		logger.Info("subsystems-already-mounted")
	}

	logger.Info("finished")

	return nil
}

func (s *CgroupStarter) isMountPoint(path string) bool {
	// append trailing slash to force symlink traversal; symlinking e.g. 'cpu'
	// to 'cpu,cpuacct' is common
	return s.CommandRunner.Run(exec.Command("mountpoint", "-q", path+"/")) == nil
}
