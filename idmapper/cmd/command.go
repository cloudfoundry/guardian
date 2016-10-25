package cmd

import (
	"os"
	"os/user"

	"code.cloudfoundry.org/idmapper/mapper"
)

func MapIds(idFilePath string, procFilePath string, mappings []string) error {
	mapFile, err := os.OpenFile(procFilePath, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	desiredMappings := mapper.Parse(mappings)

	currentUser, err := user.Current()
	if err != nil {
		return err
	}

	allowedIds, err := mapper.LoadSubids(idFilePath)
	if err != nil {
		return err
	}

	idMapper := mapper.NewIDMapper(currentUser, desiredMappings, allowedIds)
	if err := idMapper.Validate(); err != nil {
		return err
	}

	_, err = mapFile.Write(idMapper.Map())
	if err != nil {
		return err
	}

	if err := mapFile.Close(); err != nil {
		return err
	}

	return nil
}
