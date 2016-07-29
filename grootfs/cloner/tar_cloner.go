package cloner

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/cloudfoundry/gunk/command_runner"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type TarCloner struct {
	cmdRunner command_runner.CommandRunner
	idMapper  IDMapper
}

func NewTarCloner(cmdRunner command_runner.CommandRunner, idMapper IDMapper) *TarCloner {
	return &TarCloner{
		cmdRunner: cmdRunner,
		idMapper:  idMapper,
	}
}

func (c *TarCloner) Clone(logger lager.Logger, spec groot.CloneSpec) error {
	if _, err := os.Stat(spec.FromDir); err != nil {
		return fmt.Errorf("image path `%s` was not found: %s", spec.FromDir, err)
	}

	tarCmd := exec.Command("tar", "-cp", "-C", spec.FromDir, ".")
	tarCmd.Stderr = os.Stderr

	_ = os.Mkdir(spec.ToDir, 0755)
	untarPipeR, untarPipeW, _ := os.Pipe()

	untarCmd := exec.Command(os.Args[0], "untar", "--", "-xp", "-C", spec.ToDir)
	if len(spec.UIDMappings) > 0 || len(spec.GIDMappings) > 0 {
		untarCmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER,
		}
	}

	untarCmd.Stdin, _ = tarCmd.StdoutPipe()
	untarCmd.Stdout = os.Stderr
	untarCmd.Stderr = os.Stderr
	untarCmd.ExtraFiles = []*os.File{untarPipeR}
	err := c.cmdRunner.Start(untarCmd)
	if err != nil {
		return fmt.Errorf("untar: %s", err)
	}

	if len(spec.UIDMappings) > 0 {
		if err = c.idMapper.MapUIDs(untarCmd.Process.Pid, spec.UIDMappings); err != nil {
			return fmt.Errorf("uid mapping: %s", err)
		}
	}

	if len(spec.GIDMappings) > 0 {
		if err = c.idMapper.MapGIDs(untarCmd.Process.Pid, spec.GIDMappings); err != nil {
			return fmt.Errorf("gid mapping: %s", err)
		}
	}

	untarPipeW.Write([]byte{0})

	if err := tarCmd.Run(); err != nil {
		return fmt.Errorf("reading from `%s`: %s", spec.FromDir, err)
	}

	if err := c.cmdRunner.Wait(untarCmd); err != nil {
		return fmt.Errorf("writing to `%s`: %s", spec.ToDir, err)
	}

	return nil
}
