package groot

import (
	"code.cloudfoundry.org/lager"
)

type Statser struct {
	imageCloner ImageCloner
}

func IamStatser(imageCloner ImageCloner) *Statser {
	return &Statser{
		imageCloner: imageCloner,
	}
}

func (m *Statser) Stats(logger lager.Logger, id string) (VolumeStats, error) {
	logger = logger.Session("groot-stats", lager.Data{"imageID": id})
	logger.Debug("starting")
	defer logger.Debug("ending")

	stats, err := m.imageCloner.Stats(logger, id)
	if err != nil {
		logger.Error("fetching-stats", err, lager.Data{"id": id})
		return VolumeStats{}, err
	}

	return stats, nil
}
