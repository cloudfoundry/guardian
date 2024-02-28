package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"code.cloudfoundry.org/idmapper"
)

type mapping struct {
	hostID      int
	containerID int
	size        int
}

func mapIds(procFilePath string) error {
	maximusID := idmapper.Min(idmapper.MustGetMaxValidUID(), idmapper.MustGetMaxValidGID())
	currentUserUID := os.Getuid()
	if currentUserUID != maximusID {
		return fmt.Errorf("current user is %d, you can only run this as user %d", currentUserUID, maximusID)
	}

	desiredMappings := []mapping{
		{
			containerID: 0,
			hostID:      maximusID,
			size:        1,
		},
		{
			containerID: 1,
			hostID:      65536,
			size:        maximusID - 65536,
		},
	}

	return writeMapFile(procFilePath, desiredMappings)
}

func writeMapFile(procFilePath string, desiredMappings []mapping) error {
	procMappings := bytes.NewBuffer([]byte{})

	for _, mapping := range desiredMappings {
		procMappings.WriteString(
			fmt.Sprintf("%10d %10d %10d\n", mapping.containerID, mapping.hostID, mapping.size),
		)
	}

	mapFile, err := os.OpenFile(procFilePath, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	_, err = mapFile.Write(procMappings.Bytes())
	if err != nil {
		return err
	}

	return mapFile.Close()
}

func HandleCmd(idKind string) {
	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}

	procGIDMap := fmt.Sprintf("/proc/%d/%sid_map", pid, idKind)
	if err := mapIds(procGIDMap); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
}
