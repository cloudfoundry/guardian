package unpacker

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . IDMapper
type IDMapper interface {
	MapUIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error
	MapGIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error
}

type TarUnpacker struct {
	idMapper IDMapper
}

func NewTarUnpacker(idMapper IDMapper) *TarUnpacker {
	return &TarUnpacker{
		idMapper: idMapper,
	}
}

func (u *TarUnpacker) Unpack(logger lager.Logger, spec cloner.UnpackSpec) error {
	logger = logger.Session("cloning-with-tar", lager.Data{"spec": spec})
	logger.Debug("start")
	defer logger.Debug("end")

	if err := u.prepare(spec); err != nil {
		return err
	}

	untarPipeR, untarPipeW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("creating tar control pipe: %s", err)
	}

	untarBuffer, untarCmd, err := u.makeUntarCmd(spec, spec.Stream, untarPipeR)
	if err != nil {
		return err
	}

	if err := untarCmd.Start(); err != nil {
		return fmt.Errorf("starting untar: %s", err)
	}

	if err := u.setIDMappings(logger, spec, untarCmd.Process.Pid); err != nil {
		return err
	}

	if _, err := untarPipeW.Write([]byte{0}); err != nil {
		return fmt.Errorf("writing to tar control pipe: %s", err)
	}
	logger.Debug("untar-is-signaled-to-continue")

	if err := untarCmd.Wait(); err != nil {
		return fmt.Errorf("writing to `%s` %s: %s", spec.RootFSPath, err, untarBuffer.String())
	}

	return nil
}

func (u *TarUnpacker) Untar(logger lager.Logger, ctrlPipeR io.Reader, reader io.Reader, toDir string) error {
	if _, err := ctrlPipeR.Read(make([]byte, 1)); err != nil {
		return nil
	}

	cmd := exec.Command("tar", "-xp", "-C", toDir)
	cmd.Stdin = reader

	if err := u.runAndLog(cmd); err != nil {
		return fmt.Errorf("untaring: %s", err)
	}

	return nil
}

func (u *TarUnpacker) prepare(spec cloner.UnpackSpec) error {
	if err := os.Mkdir(spec.RootFSPath, 0755); err != nil {
		return fmt.Errorf("making destination directory: %s", err)
	}

	return nil
}

func (u *TarUnpacker) makeUntarCmd(spec cloner.UnpackSpec, tarStream io.ReadCloser, untarPipeR *os.File) (*bytes.Buffer, *exec.Cmd, error) {
	untarCmd := exec.Command(os.Args[0], "untar", spec.RootFSPath)
	if len(spec.UIDMappings) > 0 || len(spec.GIDMappings) > 0 {
		untarCmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER,
		}
	}

	untarCmd.Stdin = tarStream

	untarBuffer := bytes.NewBuffer([]byte{})
	untarCmd.Stdout = untarBuffer
	untarCmd.Stderr = untarBuffer
	untarCmd.ExtraFiles = []*os.File{untarPipeR}

	return untarBuffer, untarCmd, nil
}

func (u *TarUnpacker) setIDMappings(logger lager.Logger, spec cloner.UnpackSpec, untarPid int) error {
	if len(spec.UIDMappings) > 0 {
		if err := u.idMapper.MapUIDs(logger, untarPid, spec.UIDMappings); err != nil {
			return fmt.Errorf("setting uid mapping: %s", err)
		}
		logger.Debug("uid-mappings-are-set")
	}

	if len(spec.GIDMappings) > 0 {
		if err := u.idMapper.MapGIDs(logger, untarPid, spec.GIDMappings); err != nil {
			return fmt.Errorf("setting gid mapping: %s", err)
		}
		logger.Debug("gid-mappings-are-set")
	}

	return nil
}

func (u *TarUnpacker) runAndLog(cmd *exec.Cmd) error {
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %s", err, string(output))
}
