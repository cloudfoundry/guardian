package unpacker // import "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"syscall"

	"code.cloudfoundry.org/commandrunner"
	"github.com/containers/storage/pkg/reexec"
	"github.com/tscolari/lagregator"
	"github.com/urfave/cli"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

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

func init() {
	var fail = func(logger lager.Logger, message string, err error) {
		logger.Error(message, err)
		fmt.Println(err.Error())
		os.Exit(1)
	}

	reexec.Register("unpack", func() {
		cli.ErrWriter = os.Stdout
		logger := lager.NewLogger("unpack")
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

		if len(os.Args) != 4 {
			fail(logger, "parsing-command", errorspkg.New("destination directory or filesystem were not specified"))
		}

		ctrlPipeR := os.NewFile(3, "/ctrl/pipe")
		buffer := make([]byte, 1)
		logger.Debug("waiting-for-control-pipe")
		_, err := ctrlPipeR.Read(buffer)
		if err != nil {
			fail(logger, "reading-control-pipe", err)
		}
		logger.Debug("got-back-from-control-pipe")

		targetDir := os.Args[1]
		baseDirectory := os.Args[2]
		unpackStrategyJSON := os.Args[3]

		var unpackStrategy UnpackStrategy
		if err = json.Unmarshal([]byte(unpackStrategyJSON), &unpackStrategy); err != nil {
			fail(logger, "unmarshal-unpack-strategy-failed", err)
		}

		unpacker, err := NewTarUnpacker(unpackStrategy)
		if err != nil {
			fail(logger, "creating-tar-unpacker", err)
		}

		var unpackOutput base_image_puller.UnpackOutput
		if unpackOutput, err = unpacker.Unpack(logger, base_image_puller.UnpackSpec{
			Stream:        os.Stdin,
			TargetPath:    targetDir,
			BaseDirectory: baseDirectory,
		}); err != nil {
			fail(logger, "unpacking-failed", err)
		}

		json.NewEncoder(os.Stdout).Encode(unpackOutput)

		logger.Debug("unpack-command-ending")
	})
}

func NewNSIdMapperUnpacker(commandRunner commandrunner.CommandRunner, idMapper IDMapper, strategy UnpackStrategy) *NSIdMapperUnpacker {
	return &NSIdMapperUnpacker{
		commandRunner:  commandRunner,
		idMapper:       idMapper,
		unpackStrategy: strategy,
	}
}

func (u *NSIdMapperUnpacker) Unpack(logger lager.Logger, spec base_image_puller.UnpackSpec) (base_image_puller.UnpackOutput, error) {
	logger = logger.Session("ns-id-mapper-unpacking", lager.Data{"spec": spec})
	logger.Debug("starting")
	defer logger.Debug("ending")

	ctrlPipeR, ctrlPipeW, err := os.Pipe()
	if err != nil {
		return base_image_puller.UnpackOutput{}, errorspkg.Wrap(err, "creating tar control pipe")
	}

	unpackStrategyJSON, err := json.Marshal(&u.unpackStrategy)
	if err != nil {
		logger.Error("unmarshal-unpack-strategy-failed", err)
		return base_image_puller.UnpackOutput{}, errorspkg.Wrap(err, "unmarshal unpack strategy")
	}

	unpackCmd := reexec.Command("unpack", spec.TargetPath, spec.BaseDirectory, string(unpackStrategyJSON))
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

	logger.Debug("starting-unpack-command", lager.Data{
		"path": unpackCmd.Path,
		"args": unpackCmd.Args,
	})
	if err := u.commandRunner.Start(unpackCmd); err != nil {
		return base_image_puller.UnpackOutput{}, errorspkg.Wrap(err, "starting unpack command")
	}
	logger.Debug("unpack-command-is-started")

	if err := u.setIDMappings(logger, spec, unpackCmd.Process.Pid); err != nil {
		_ = ctrlPipeW.Close()
		return base_image_puller.UnpackOutput{}, err
	}

	if _, err := ctrlPipeW.Write([]byte{0}); err != nil {
		return base_image_puller.UnpackOutput{}, errorspkg.Wrap(err, "writing to tar control pipe")
	}
	logger.Debug("unpack-command-is-signaled-to-continue")

	logger.Debug("waiting-for-unpack-command")
	if err := u.commandRunner.Wait(unpackCmd); err != nil {
		return base_image_puller.UnpackOutput{}, errorspkg.Errorf(outBuffer.String())
	}
	logger.Debug("unpack-command-done")

	var unpackOutput base_image_puller.UnpackOutput
	if err := json.NewDecoder(outBuffer).Decode(&unpackOutput); err != nil {
		logger.Error("invalid-output-from-unpack", err)
		return base_image_puller.UnpackOutput{}, errorspkg.Wrapf(err, "invalid unpack output (%s)", err.Error())
	}

	return unpackOutput, nil
}

func (u *NSIdMapperUnpacker) setIDMappings(logger lager.Logger, spec base_image_puller.UnpackSpec, untarPid int) error {
	if len(spec.UIDMappings) > 0 {
		if err := u.idMapper.MapUIDs(logger, untarPid, spec.UIDMappings); err != nil {
			return errorspkg.Wrap(err, "setting uid mapping")
		}
		logger.Debug("uid-mappings-are-set")
	}

	if len(spec.GIDMappings) > 0 {
		if err := u.idMapper.MapGIDs(logger, untarPid, spec.GIDMappings); err != nil {
			return errorspkg.Wrap(err, "setting gid mapping")
		}
		logger.Debug("gid-mappings-are-set")
	}

	return nil
}
