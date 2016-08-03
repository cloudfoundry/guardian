package cloner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . IDMapper
type IDMapper interface {
	MapUIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error
	MapGIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error
}

type TarCloner struct {
	idMapper IDMapper
}

func NewTarCloner(idMapper IDMapper) *TarCloner {
	return &TarCloner{
		idMapper: idMapper,
	}
}

func (c *TarCloner) Clone(logger lager.Logger, spec groot.CloneSpec) error {
	logger = logger.Session("cloning-with-tar", lager.Data{"spec": spec})
	logger.Debug("start")
	defer logger.Debug("end")

	if _, err := os.Stat(spec.FromDir); err != nil {
		return fmt.Errorf("image path `%s` was not found: %s", spec.FromDir, err)
	}

	if err := os.Mkdir(spec.ToDir, 0755); err != nil {
		return fmt.Errorf("making destination directory: %s", err)
	}

	tarCmd := exec.Command("tar", "-cp", "-C", spec.FromDir, ".")
	tarCmd.Stderr = os.Stderr

	untarPipeR, untarPipeW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("creating tar control pipe: %s", err)
	}

	untarCmd := exec.Command(os.Args[0], "untar", spec.ToDir)
	if len(spec.UIDMappings) > 0 || len(spec.GIDMappings) > 0 {
		untarCmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER,
		}
	}

	untarCmd.Stdin, err = tarCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating tar stdout pipe: %s", err)
	}
	untarCmd.Stdout = os.Stderr
	untarCmd.Stderr = os.Stderr
	untarCmd.ExtraFiles = []*os.File{untarPipeR}
	logger.Debug("starting-untar")
	if err := untarCmd.Start(); err != nil {
		return fmt.Errorf("starting untar: %s", err)
	}

	if err := c.setIDMappings(logger, spec, untarCmd.Process.Pid); err != nil {
		return err
	}

	if _, err := untarPipeW.Write([]byte{0}); err != nil {
		return fmt.Errorf("writing to tar control pipe: %s", err)
	}
	logger.Debug("untar-is-signaled-to-continue")

	if err := tarCmd.Run(); err != nil {
		return fmt.Errorf("reading from `%s`: %s", spec.FromDir, err)
	}
	logger.Debug("tar-is-done")

	if err := untarCmd.Wait(); err != nil {
		return fmt.Errorf("writing to `%s`: %s", spec.ToDir, err)
	}

	return nil
}

func (c *TarCloner) setIDMappings(logger lager.Logger, spec groot.CloneSpec, untarPid int) error {
	if len(spec.UIDMappings) > 0 {
		if err := c.idMapper.MapUIDs(logger, untarPid, spec.UIDMappings); err != nil {
			return fmt.Errorf("setting uid mapping: %s", err)
		}
		logger.Debug("uid-mappings-are-set")
	}

	if len(spec.GIDMappings) > 0 {
		if err := c.idMapper.MapGIDs(logger, untarPid, spec.GIDMappings); err != nil {
			return fmt.Errorf("setting gid mapping: %s", err)
		}
		logger.Debug("gid-mappings-are-set")
	}

	return nil
}

func (c *TarCloner) Untar(logger lager.Logger, ctrlPipeR io.Reader, reader io.Reader, toDir string) error {
	if _, err := ctrlPipeR.Read(make([]byte, 1)); err != nil {
		return nil
	}

	cmd := exec.Command("tar", "-xp", "-C", toDir)
	cmd.Stdin = reader
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
