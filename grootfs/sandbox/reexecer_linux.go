// +build linux

package sandbox // import "code.cloudfoundry.org/grootfs/sandbox"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/containers/storage/pkg/reexec"
	"github.com/tscolari/lagregator"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

const (
	reexecChrootDirEnv  = "REEXEC_CHROOT_DIR"
	reexecExtraFilesEnv = "REEXEC_EXTRA_FILES"
)

var registeredCommands = make(map[string]struct{})

func Register(commandName string, action func(logger lager.Logger, extraFiles []*os.File, args ...string) error) {
	registeredCommands[commandName] = struct{}{}

	reexecWrapperName := commandName + "-wrapper"

	reexec.Register(reexecWrapperName, func() {
		logger := lager.NewLogger(reexecWrapperName)
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))
		logger.Debug("wrapper-command-starting", lager.Data{"reexecWrapperName": reexecWrapperName})
		defer logger.Debug("wrapper-command-ending")

		if err := waitForIDMappings(logger, reexecWrapperName); err != nil {
			fail(logger, "waiting-for-id-mappings", err)
		}

		reexecCmdArgs := []string{commandName}
		reexecCmdArgs = append(reexecCmdArgs, os.Args[1:]...)
		reexecCmd := reexec.Command(reexecCmdArgs...)
		reexecCmd.Stdin = os.Stdin
		reexecCmd.Stdout = os.Stdout
		reexecCmd.Stderr = os.Stderr
		reexecCmd.Env = append(reexecCmd.Env, fmt.Sprintf("%s=%s", reexecExtraFilesEnv, os.Getenv(reexecExtraFilesEnv)))
		if chrootDir, changeRoot := os.LookupEnv(reexecChrootDirEnv); changeRoot {
			reexecCmd.Env = append(reexecCmd.Env, fmt.Sprintf("%s=%s", reexecChrootDirEnv, chrootDir))
		}

		if err := reexecCmd.Run(); err != nil {
			fail(logger, "running-reexec-command", err)
		}
	})

	reexec.Register(commandName, func() {
		logger := lager.NewLogger(commandName)
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))
		logger.Debug(fmt.Sprintf("%s-starting", commandName))
		defer logger.Debug(fmt.Sprintf("%s-ending", commandName))

		var extraFileNames []string
		if err := json.Unmarshal([]byte(os.Getenv(reexecExtraFilesEnv)), &extraFileNames); err != nil {
			fail(logger, "unmarshaling extra files", err)
		}

		extraFiles := []*os.File{}
		for _, extraFileName := range extraFileNames {
			f, err := os.Open(filepath.Clean(extraFileName))
			defer f.Close()

			if err != nil {
				fail(logger, "opening extra file: "+extraFileName, err)
			}
			extraFiles = append(extraFiles, f)
		}

		if chrootDir, changeRoot := os.LookupEnv(reexecChrootDirEnv); changeRoot {
			logger.Debug("chrooting into: " + chrootDir)
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			if err := chroot(chrootDir); err != nil {
				fail(logger, "chroot", err)
			}
		}

		logger.Debug("running-action")
		if err := action(logger, extraFiles, os.Args[1:]...); err != nil {
			fail(logger, "running-action", err)
		}
		logger.Debug("action-completed")
	})
}

func (r *reexecer) Reexec(commandName string, spec groot.ReexecSpec) ([]byte, error) {
	r.logger.Debug("reexec-starting")
	defer r.logger.Debug("reexec-ending")

	if !isRegistered(commandName) {
		return nil, fmt.Errorf("unregistered command: %s", commandName)
	}

	ctrlPipeR, ctrlPipeW, err := os.Pipe()
	if err != nil {
		return nil, errorspkg.Wrap(err, "creating reexec control pipe")
	}

	reexecCommandName := commandName
	if spec.CloneUserns {
		reexecCommandName = commandName + "-wrapper"
	}
	cmdArgs := []string{reexecCommandName}
	cmdArgs = append(cmdArgs, spec.Args...)
	reexecCmd := reexec.Command(cmdArgs...)
	reexecCmd.Stdin = spec.Stdin

	if spec.CloneUserns {
		reexecCmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER,
		}
	}

	outBuffer := bytes.NewBuffer([]byte{})
	reexecCmd.Stdout = outBuffer
	reexecCmd.Stderr = lagregator.NewRelogger(r.logger)
	reexecCmd.ExtraFiles = []*os.File{ctrlPipeR}
	if spec.ChrootDir != "" {
		reexecCmd.Env = append(reexecCmd.Env, fmt.Sprintf("%s=%s", reexecChrootDirEnv, spec.ChrootDir))
	}

	extraFilesJSON, err := json.Marshal(spec.ExtraFiles)
	if err != nil {
		return nil, errorspkg.Wrap(err, "marshaling extra files")
	}
	reexecCmd.Env = append(reexecCmd.Env, fmt.Sprintf("%s=%s", reexecExtraFilesEnv, extraFilesJSON))

	r.logger.Debug("starting-reexec-command", lager.Data{
		"name": reexecCommandName,
		"path": reexecCmd.Path,
		"spec": spec,
	})
	if err := reexecCmd.Start(); err != nil {
		return nil, errorspkg.Wrap(err, "starting reexec command")
	}
	r.logger.Debug("reexec-command-is-started")

	if spec.CloneUserns {
		if err := r.setIDMappings(reexecCmd.Process.Pid); err != nil {
			_ = ctrlPipeW.Close()
			return nil, err
		}

		if _, err := ctrlPipeW.Write([]byte{0}); err != nil {
			return nil, errorspkg.Wrap(err, "writing to control pipe")
		}
		r.logger.Debug("reexec-command-is-signaled-to-continue")
	}

	r.logger.Debug("waiting-for-reexec-command")
	if err := reexecCmd.Wait(); err != nil {
		return nil, errorspkg.Errorf("waiting for the reexec command failed: %s: %s", outBuffer.String(), err)
	}
	r.logger.Debug("reexec-command-done")

	return outBuffer.Bytes(), nil
}

func (r *reexecer) setIDMappings(reexecPid int) error {
	if len(r.idMappings.UIDMappings) > 0 {
		if err := r.idMapper.MapUIDs(r.logger, reexecPid, r.idMappings.UIDMappings); err != nil {
			return errorspkg.Wrap(err, "setting uid mapping")
		}
		r.logger.Debug("uid-mappings-are-set", lager.Data{"uidMappings": r.idMappings.UIDMappings})
	}

	if len(r.idMappings.GIDMappings) > 0 {
		if err := r.idMapper.MapGIDs(r.logger, reexecPid, r.idMappings.GIDMappings); err != nil {
			return errorspkg.Wrap(err, "setting gid mapping")
		}
		r.logger.Debug("gid-mappings-are-set", lager.Data{"gidM": r.idMappings.GIDMappings})
	}

	return nil
}

func waitForIDMappings(logger lager.Logger, commandName string) error {
	ctrlPipeR := os.NewFile(3, fmt.Sprintf("/ctrl/%s-pipe", commandName))
	buffer := make([]byte, 1)
	logger.Debug("waiting-for-control-pipe")
	defer logger.Debug("got-back-from-control-pipe")
	_, err := ctrlPipeR.Read(buffer)
	return err
}

func chroot(path string) error {
	if err := syscall.Chroot(path); err != nil {
		return fmt.Errorf("could not chroot: %s", err.Error())
	}

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("could not chdir: %s", err.Error())
	}

	return nil
}

func fail(logger lager.Logger, message string, err error) {
	logger.Error(message, err)
	fmt.Println(err.Error())
	os.Exit(1)
}

func isRegistered(commandName string) bool {
	_, ok := registeredCommands[commandName]
	return ok
}
