package cgroups

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/lager"
)

const cgroupsHeader = "#subsys_name hierarchy num_cgroups enabled"

type CgroupsFormatError struct {
	Content string
}

func (err CgroupsFormatError) Error() string {
	return fmt.Sprintf("unknown /proc/cgroups format: %s", err.Content)
}

func NewStarter(
	logger lager.Logger,
	procCgroupReader io.ReadCloser,
	procSelfCgroupReader io.ReadCloser,
	procSelfMountinfoReader io.ReadCloser,
	cgroupMountpoint string,
	gardenCgroup string,
	allowedDevices []specs.LinuxDeviceCgroup,
	runner commandrunner.CommandRunner,
	chowner Chowner,
) *CgroupStarter {
	return &CgroupStarter{
		CgroupPath:        cgroupMountpoint,
		GardenCgroup:      gardenCgroup,
		ProcCgroups:       procCgroupReader,
		ProcSelfCgroups:   procSelfCgroupReader,
		ProcSelfMountinfo: procSelfMountinfoReader,
		AllowedDevices:    allowedDevices,
		CommandRunner:     runner,
		Logger:            logger,
		Chowner:           chowner,
	}
}

type CgroupStarter struct {
	CgroupPath     string
	GardenCgroup   string
	AllowedDevices []specs.LinuxDeviceCgroup
	CommandRunner  commandrunner.CommandRunner

	ProcCgroups       io.ReadCloser
	ProcSelfCgroups   io.ReadCloser
	ProcSelfMountinfo io.ReadCloser

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

	mountPoint, err := s.isMountPoint(s.CgroupPath)
	if err != nil {
		return err
	}
	if !mountPoint {
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

		gardenCgroupPath := filepath.Join(s.CgroupPath, subsystem, dirToCreate)
		if err := s.createGardenCgroup(logger, gardenCgroupPath); err != nil {
			return err
		}

		if subsystem == "devices" {
			if err := s.modifyAllowedDevices(gardenCgroupPath, s.AllowedDevices); err != nil {
				return err
			}
		}

		if err := s.Chowner.RecursiveChown(gardenCgroupPath); err != nil {
			return err
		}
	}

	return nil
}

func (s *CgroupStarter) modifyAllowedDevices(dir string, devices []specs.LinuxDeviceCgroup) error {
	if has, err := hasSubdirectories(dir); err != nil {
		return err
	} else if has {
		return nil
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "devices.deny"), []byte("a"), 0770); err != nil {
		return err
	}
	for _, device := range devices {
		data := fmt.Sprintf("%s %s:%s %s", device.Type, s.deviceNumberString(device.Major), s.deviceNumberString(device.Minor), device.Access)
		if err := s.setDeviceCgroup(dir, "devices.allow", data); err != nil {
			return err
		}
	}

	return nil
}

func hasSubdirectories(dir string) (bool, error) {
	dirs, err := ioutil.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, fileInfo := range dirs {
		if fileInfo.Mode().IsDir() {
			return true, nil
		}
	}
	return false, nil
}

func (d *CgroupStarter) setDeviceCgroup(dir, file, data string) error {
	if err := ioutil.WriteFile(filepath.Join(dir, file), []byte(data), 0); err != nil {
		return fmt.Errorf("failed to write %s to %s: %v", data, file, err)
	}

	return nil
}

func (s *CgroupStarter) deviceNumberString(number *int64) string {
	if *number == -1 {
		return "*"
	}
	return fmt.Sprint(*number)
}

func (s *CgroupStarter) createGardenCgroup(log lager.Logger, gardenCgroupPath string) error {
	log = log.Session("creating-garden-cgroup", lager.Data{"gardenCgroup": gardenCgroupPath})
	log.Info("started")
	defer log.Info("finished")

	if err := os.MkdirAll(gardenCgroupPath, 0755); err != nil {
		return err
	}

	return os.Chmod(gardenCgroupPath, 0755)
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

	existingCgroupHierarchy, err := s.existingCgroupHierarchy()
	if err != nil {
		return groupings, err
	}

	scanner := bufio.NewScanner(s.ProcSelfCgroups)
	for scanner.Scan() {
		segs := strings.Split(scanner.Text(), ":")
		if len(segs) != 3 {
			continue
		}

		subsystems := strings.Split(segs[1], ",")
		for _, subsystem := range subsystems {
			path := segs[2]
			if existingPath, ok := existingCgroupHierarchy[subsystem]; ok {
				// If the existing cgroup mountpoint is mounting a prefix of the
				// desired path, we're inside of a cgroup ourselves.  When
				// garden creates further cgroups (directories), we should only
				// do so relative to our current mount point.
				if newPath, err := filepath.Rel(existingPath.Path, path); err == nil {
					path = filepath.Clean("/" + newPath)
				}
			}
			groupings[subsystem] = group{segs[1], path}
		}
	}

	return groupings, scanner.Err()
}

// existingCgroupHierarchy returns the paths of the namespaces in effect
// relative to the root namespace.
func (s *CgroupStarter) existingCgroupHierarchy() (map[string]group, error) {
	groupings := map[string]group{}

	scanner := bufio.NewScanner(s.ProcSelfMountinfo)
	for scanner.Scan() {
		segs := strings.Split(scanner.Text(), " ")
		// Format: https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/Documentation/filesystems/proc.txt?h=v4.15#n1680
		// We need to find the field that signifies the end of optional fields
		optionalEnd := -1
		for idx, seg := range segs {
			if seg == "-" {
				optionalEnd = idx
				break
			}
		}
		if optionalEnd < 6 || len(segs) < (optionalEnd+2) || segs[optionalEnd+1] != "cgroup" {
			continue
		}
		subsystems := path.Base(segs[4])
		for _, subsystem := range strings.Split(subsystems, ",") {
			groupings[subsystem] = group{subsystems, segs[3]}
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

	mountPoint, err := s.isMountPoint(cgroupPath)
	if err != nil {
		return err
	}
	if !mountPoint {
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

func (s *CgroupStarter) isMountPoint(path string) (bool, error) {
	// append trailing slash to force symlink traversal; symlinking e.g. 'cpu'
	// to 'cpu,cpuacct' is common
	cmd := exec.Command("mountpoint", "-q", path+"/")

	if err := s.CommandRunner.Start(cmd); err != nil {
		return false, err
	}

	return s.CommandRunner.Wait(cmd) == nil, nil
}
