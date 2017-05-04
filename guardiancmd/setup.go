package guardiancmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/sysinfo"
	"code.cloudfoundry.org/idmapper"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
)

type SetupCommand struct {
	LogLevel LagerFlag
	Logger   lager.Logger

	RootlessForUID uint32 `long:"experimental-allow-rootless-for-uid" default:"0" description:"set permissions to allow (non-root) user to run gdn"`

	Tag  string `hidden:"true" long:"tag" description:"Optional 2-character identifier used for namespacing global configuration."`
	Root string `hidden:"true" long:"runc-root" default:"/run/runc" description:"root directory for storage of container state (this should be located in tmpfs)"`
}

func (cmd *SetupCommand) Execute(args []string) error {
	cmd.Logger, _ = cmd.LogLevel.Logger("guardian-setup")

	cgroupsMountpoint := "/sys/fs/cgroup"
	if cmd.Tag != "" {
		cgroupsMountpoint = filepath.Join(os.TempDir(), fmt.Sprintf("cgroups-%s", cmd.Tag))
	}
	cgroupStarter := rundmc.NewStarter(cmd.Logger, mustOpen("/proc/cgroups"), mustOpen("/proc/self/cgroup"), cgroupsMountpoint, linux_command_runner.New())
	if err := cgroupStarter.Start(); err != nil {
		return err
	}

	if cmd.RootlessForUID == 0 {
		return nil
	}

	if err := cmd.createDirForUser(cmd.Root); err != nil {
		return err
	}

	if err := cmd.createDirForUser(fmt.Sprintf("/var/run/user/%d/gdn", cmd.RootlessForUID)); err != nil {
		return err
	}

	subuidFileContents, err := ioutil.ReadFile("/etc/subuid")
	if err != nil {
		return err
	}
	subgidFileContents, err := ioutil.ReadFile("/etc/subgid")
	if err != nil {
		return err
	}
	if !sysinfo.UidCanMapExactRange(string(subuidFileContents), cmd.RootlessForUID, 0, uint32(idmapper.MustGetMaxValidUID()+1)) {
		fmt.Printf("WARNING: uid %d does not have permission to map the entire UID range\n", cmd.RootlessForUID)
	}
	if !sysinfo.UidCanMapExactRange(string(subgidFileContents), cmd.RootlessForUID, 0, uint32(idmapper.MustGetMaxValidGID()+1)) {
		fmt.Printf("WARNING: uid %d does not have permission to map the entire GID range\n", cmd.RootlessForUID)
	}

	return nil
}

func (cmd *SetupCommand) createDirForUser(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}

	if err := os.Chown(path, int(cmd.RootlessForUID), 0); err != nil {
		return err
	}

	return nil
}
