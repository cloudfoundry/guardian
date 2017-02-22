package unpacker // import "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"syscall"

	"code.cloudfoundry.org/commandrunner"
	"github.com/docker/docker/pkg/reexec"
	"github.com/tscolari/lagregator"
	"github.com/urfave/cli"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

func init() {
	reexec.Register("unpack-wrapper", func() {
		cli.ErrWriter = os.Stdout
		logger := lager.NewLogger("unpack-wrapper")
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

		if len(os.Args) != 3 {
			logger.Error("parsing-command", errors.New("destination directory or filesystem were not specified"))
			os.Exit(1)
		}

		ctrlPipeR := os.NewFile(3, "/ctrl/pipe")
		buffer := make([]byte, 1)
		logger.Debug("waiting-for-control-pipe")
		_, err := ctrlPipeR.Read(buffer)
		if err != nil {
			logger.Error("reading-control-pipe", err)
			os.Exit(1)
		}
		logger.Debug("got-back-from-control-pipe")

		// Once all id mappings are set, we need to spawn the untar function
		// in a child proccess, so it can make use of it
		targetDir := os.Args[1]
		unpackStrategyJson := os.Args[2]
		cmd := reexec.Command("unpack", targetDir, unpackStrategyJson)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		logger.Debug("starting-unpack", lager.Data{
			"path": cmd.Path,
			"args": cmd.Args,
		})
		if err := cmd.Run(); err != nil {
			logger.Error("unpack-command-failed", err)
			os.Exit(1)
		}
		logger.Debug("unpack-command-done")
	})
}

//go:generate counterfeiter . IDMapper
type IDMapper interface {
	MapUIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error
	MapGIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error
}

type NSIdMapperUnpacker struct {
	commandRunner  commandrunner.CommandRunner
	idMapper       IDMapper
	unpackStrategy UnpackStrategy
}

func NewNSIdMapperUnpacker(commandRunner commandrunner.CommandRunner, idMapper IDMapper, strategy UnpackStrategy) *NSIdMapperUnpacker {
	return &NSIdMapperUnpacker{
		commandRunner:  commandRunner,
		idMapper:       idMapper,
		unpackStrategy: strategy,
	}
}

func (u *NSIdMapperUnpacker) Unpack(logger lager.Logger, spec base_image_puller.UnpackSpec) error {
	logger = logger.Session("ns-id-mapper-unpacking", lager.Data{"spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	ctrlPipeR, ctrlPipeW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("creating tar control pipe: %s", err)
	}

	unpackStrategyJson, err := json.Marshal(&u.unpackStrategy)
	if err != nil {
		logger.Error("unmarshal-unpack-strategy-failed", err)
		return fmt.Errorf("unmarshal unpack strategy: %s", err)
	}

	unpackCmd := reexec.Command("unpack-wrapper", spec.TargetPath, string(unpackStrategyJson))
	unpackCmd.Stdin = spec.Stream
	if len(spec.UIDMappings) > 0 || len(spec.GIDMappings) > 0 {
		unpackCmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER,
		}
	}

	outBuffer := bytes.NewBuffer([]byte{})
	unpackCmd.Stdout = outBuffer
	unpackCmd.Stderr = lagregator.NewRelogger(logger)
	unpackCmd.ExtraFiles = []*os.File{ctrlPipeR}

	logger.Debug("starting-unpack-wrapper-command", lager.Data{
		"path": unpackCmd.Path,
		"args": unpackCmd.Args,
	})
	if err := u.commandRunner.Start(unpackCmd); err != nil {
		return fmt.Errorf("starting unpack command: %s", err)
	}
	logger.Debug("unpack-wrapper-command-is-started")

	if err := u.setIDMappings(logger, spec, unpackCmd.Process.Pid); err != nil {
		ctrlPipeW.Close()
		return err
	}

	if _, err := ctrlPipeW.Write([]byte{0}); err != nil {
		return fmt.Errorf("writing to tar control pipe: %s", err)
	}
	logger.Debug("unpack-wrapper-command-is-signaled-to-continue")

	logger.Debug("waiting-for-unpack-wrapper-command")
	if err := u.commandRunner.Wait(unpackCmd); err != nil {
		return fmt.Errorf(outBuffer.String())
	}
	logger.Debug("unpack-wrapper-command-done")

	return nil
}

func (u *NSIdMapperUnpacker) setIDMappings(logger lager.Logger, spec base_image_puller.UnpackSpec, untarPid int) error {
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
