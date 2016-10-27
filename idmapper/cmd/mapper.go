package cmd

import (
	"bytes"
	"fmt"
	"os"

	"code.cloudfoundry.org/idmapper"
)

type mapping struct {
	hostID      int
	containerID int
	size        int
}

func MapIds(procFilePath string) error {
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
			hostID:      1,
			size:        maximusID - 1,
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
