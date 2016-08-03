package cloner

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/cloudfoundry/gunk/command_runner"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type CommandIDMapper struct {
	cmdRunner command_runner.CommandRunner
}

func NewIDMapper(cmdRunner command_runner.CommandRunner) *CommandIDMapper {
	return &CommandIDMapper{
		cmdRunner: cmdRunner,
	}
}

func (im *CommandIDMapper) MapUIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error {
	return im.execute("newuidmap", pid, mappings)
}

func (im *CommandIDMapper) MapGIDs(logger lager.Logger, pid int, mappings []groot.IDMappingSpec) error {
	return im.execute("newgidmap", pid, mappings)
}

func (im *CommandIDMapper) execute(command string, pid int, mappings []groot.IDMappingSpec) error {
	mappingArgs := append([]string{strconv.Itoa(pid)}, im.idMappingsToArgs(mappings)...)
	mapCmd := exec.Command(command, mappingArgs...)

	buffer := bytes.NewBuffer([]byte{})
	mapCmd.Stdout = buffer
	mapCmd.Stderr = buffer

	if err := im.cmdRunner.Run(mapCmd); err != nil {
		return fmt.Errorf("%s %s: %s", command, err, buffer.String())
	}

	return nil
}

func (im *CommandIDMapper) idMappingsToArgs(idMappings []groot.IDMappingSpec) (args []string) {
	for _, idMapping := range idMappings {
		args = append(args, im.idMappingToArgs(idMapping)...)
	}

	return args
}

func (im *CommandIDMapper) idMappingToArgs(idMapping groot.IDMappingSpec) []string {
	return []string{
		strconv.Itoa(idMapping.NamespaceID),
		strconv.Itoa(idMapping.HostID),
		strconv.Itoa(idMapping.Size),
	}
}
