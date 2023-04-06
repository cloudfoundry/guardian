package groot

import (
	"code.cloudfoundry.org/lager/v3"
)

type Statser struct {
	imageManager ImageManager
}

func IamStatser(imageManager ImageManager) *Statser {
	return &Statser{
		imageManager: imageManager,
	}
}

func (m *Statser) Stats(logger lager.Logger, id string) (VolumeStats, error) {
	logger = logger.Session("groot-stats", lager.Data{"imageID": id})
	logger.Debug("starting")
	defer logger.Debug("ending")

	stats, err := m.imageManager.Stats(logger, id)
	if err != nil {
		logger.Error("fetching-stats", err, lager.Data{"id": id})
		return VolumeStats{}, err
	}

	return stats, nil
}
