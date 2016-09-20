package unpacker

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/lager/chug"
	"github.com/docker/docker/pkg/reexec"
	"github.com/urfave/cli"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/image_puller"
	"code.cloudfoundry.org/lager"
)

func init() {
	reexec.Register("unpack-wrapper", func() {
		cli.ErrWriter = os.Stdout
		logger := lager.NewLogger("unpack-wrapper")
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

		if len(os.Args) != 2 {
			logger.Error("parsing-command", errors.New("destination directory was not specified"))
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
		cmd := reexec.Command("unpack", os.Args[1])
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

	reexec.Register("unpack", func() {
		cli.ErrWriter = os.Stdout
		logger := lager.NewLogger("unpack")
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

		rootFSPath := os.Args[1]
		unpacker := NewTarUnpacker()
		if err := unpacker.Unpack(logger, image_puller.UnpackSpec{
			Stream:     os.Stdin,
			TargetPath: rootFSPath,
		}); err != nil {
			logger.Error("unpacking-failed", err)
			fmt.Println(err.Error())
			os.Exit(1)
		}
	})
}

//go:generate counterfeiter . IDMapper
type IDMapper interface {
	MapUIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error
	MapGIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error
}

type NamespacedUnpacker struct {
	commandRunner commandrunner.CommandRunner
	idMapper      IDMapper
}

func NewNamespacedUnpacker(commandRunner commandrunner.CommandRunner, idMapper IDMapper) *NamespacedUnpacker {
	return &NamespacedUnpacker{
		commandRunner: commandRunner,
		idMapper:      idMapper,
	}
}

func (u *NamespacedUnpacker) Unpack(logger lager.Logger, spec image_puller.UnpackSpec) error {
	logger = logger.Session("namespaced-unpacking", lager.Data{"spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	ctrlPipeR, ctrlPipeW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("creating tar control pipe: %s", err)
	}

	unpackCmd := reexec.Command("unpack-wrapper", spec.TargetPath)
	unpackCmd.Stdin = spec.Stream
	if len(spec.UIDMappings) > 0 || len(spec.GIDMappings) > 0 {
		unpackCmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER,
		}
	}

	outBuffer := bytes.NewBuffer([]byte{})
	unpackCmd.Stdout = outBuffer
	logBuffer := bytes.NewBuffer([]byte{})
	unpackCmd.Stderr = logBuffer
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
	u.relogStream(logger, logBuffer)

	return nil
}

func (u *NamespacedUnpacker) setIDMappings(logger lager.Logger, spec image_puller.UnpackSpec, untarPid int) error {
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

func (u *NamespacedUnpacker) relogStream(logger lager.Logger, stream io.Reader) {
	entries := make(chan chug.Entry, 1000)
	chug.Chug(stream, entries)

	for entry := range entries {
		if entry.IsLager {
			logger.Debug(entry.Log.Message, lager.Data{
				"timestamp": entry.Log.Timestamp,
				"source":    entry.Log.Source,
				"log_level": entry.Log.LogLevel,
				"data":      entry.Log.Data,
			})
		}
	}
}
