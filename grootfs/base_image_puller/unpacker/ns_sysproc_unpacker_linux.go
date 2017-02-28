package unpacker // import "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"

import (
	"bytes"
	"encoding/json"
	"syscall"

	"code.cloudfoundry.org/commandrunner"
	"github.com/docker/docker/pkg/reexec"
	errorspkg "github.com/pkg/errors"
	"github.com/tscolari/lagregator"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type NSSysProcUnpacker struct {
	commandRunner  commandrunner.CommandRunner
	unpackStrategy UnpackStrategy
}

func NewNSSysProcUnpacker(commandRunner commandrunner.CommandRunner, unpackStrategy UnpackStrategy) *NSSysProcUnpacker {
	return &NSSysProcUnpacker{
		commandRunner:  commandRunner,
		unpackStrategy: unpackStrategy,
	}
}

func (u *NSSysProcUnpacker) Unpack(logger lager.Logger, spec base_image_puller.UnpackSpec) error {
	logger = logger.Session("ns-sysproc-unpacking", lager.Data{"spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	unpackStrategyJson, err := json.Marshal(&u.unpackStrategy)
	if err != nil {
		logger.Error("unmarshal-unpack-strategy-failed", err)
		return errorspkg.Wrap(err, "unmarshal unpack strategy")
	}

	unpackCmd := reexec.Command("unpack", spec.TargetPath, string(unpackStrategyJson))
	unpackCmd.Stdin = spec.Stream
	if len(spec.UIDMappings) > 0 || len(spec.GIDMappings) > 0 {
		unpackCmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags:  syscall.CLONE_NEWUSER,
			UidMappings: u.parseMappings(spec.UIDMappings),
			GidMappings: u.parseMappings(spec.GIDMappings),
			Credential: &syscall.Credential{
				Uid: 0,
				Gid: 0,
			},
		}
	}

	outBuffer := bytes.NewBuffer([]byte{})
	unpackCmd.Stdout = outBuffer
	unpackCmd.Stderr = lagregator.NewRelogger(logger)

	logger.Debug("starting-unpack-command", lager.Data{
		"path": unpackCmd.Path,
		"args": unpackCmd.Args,
	})

	if err := u.commandRunner.Run(unpackCmd); err != nil {
		return errorspkg.Wrapf(err, "unpack command: %s", outBuffer.String())
	}
	logger.Debug("unpack-command-done")

	return nil
}

func (u *NSSysProcUnpacker) parseMappings(grootMappings []groot.IDMappingSpec) []syscall.SysProcIDMap {
	mappings := []syscall.SysProcIDMap{}

	for _, mapping := range grootMappings {
		mappings = append(mappings, syscall.SysProcIDMap{
			HostID:      mapping.HostID,
			ContainerID: mapping.NamespaceID,
			Size:        mapping.Size,
		})
	}

	return mappings
}
