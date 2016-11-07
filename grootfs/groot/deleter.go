package groot

import (
	"fmt"

	"code.cloudfoundry.org/lager"
)

type Deleter struct {
	imageCloner       ImageCloner
	dependencyManager DependencyManager
}

func IamDeleter(imageCloner ImageCloner, dependencyManager DependencyManager) *Deleter {
	return &Deleter{
		imageCloner:       imageCloner,
		dependencyManager: dependencyManager,
	}
}

func (d *Deleter) Delete(logger lager.Logger, id string) error {
	logger = logger.Session("groot-deleting", lager.Data{"imageID": id})
	logger.Info("start")
	defer logger.Info("end")

	err := d.imageCloner.Destroy(logger, id)
	imageRefName := fmt.Sprintf(ImageReferenceFormat, id)
	if derErr := d.dependencyManager.Deregister(imageRefName); derErr != nil {
		logger.Error("failed-to-deregister-dependencies", derErr)
	}

	return err
}
