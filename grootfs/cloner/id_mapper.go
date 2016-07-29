package cloner

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/cloudfoundry/gunk/command_runner"

	"code.cloudfoundry.org/grootfs/groot"
)

//go:generate counterfeiter . IDMapper
type IDMapper interface {
	MapUIDs(pid int, mappings []groot.IDMappingSpec) error
	MapGIDs(pid int, mappings []groot.IDMappingSpec) error
}

type idMapper struct {
	cmdRunner command_runner.CommandRunner
}

func NewIDMapper(cmdRunner command_runner.CommandRunner) *idMapper {
	return &idMapper{
		cmdRunner: cmdRunner,
	}
}

func (im *idMapper) MapUIDs(pid int, mappings []groot.IDMappingSpec) error {
	return im.execute("newuidmap", pid, mappings)
}

func (im *idMapper) MapGIDs(pid int, mappings []groot.IDMappingSpec) error {
	return im.execute("newgidmap", pid, mappings)
}

func (im *idMapper) execute(command string, pid int, mappings []groot.IDMappingSpec) error {
	mappingArgs := append([]string{strconv.Itoa(pid)}, im.idMappingsToArgs(mappings)...)
	mapCmd := exec.Command(command, mappingArgs...)
	mapCmd.Stdout = os.Stderr
	mapCmd.Stderr = os.Stderr

	if err := im.cmdRunner.Run(mapCmd); err != nil {
		return fmt.Errorf("%s: %s", command, err)
	}

	return nil
}

func (im *idMapper) idMappingsToArgs(idMappings []groot.IDMappingSpec) (args []string) {
	for _, idMapping := range idMappings {
		args = append(args, im.idMappingToArgs(idMapping)...)
	}

	return args
}

func (im *idMapper) idMappingToArgs(idMapping groot.IDMappingSpec) []string {
	return []string{
		strconv.Itoa(idMapping.NamespaceID),
		strconv.Itoa(idMapping.HostID),
		strconv.Itoa(idMapping.Size),
	}
}
