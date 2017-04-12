package store

import (
	"code.cloudfoundry.org/lager"
)

type StoreMeasurer struct {
	storePath string
}

func NewStoreMeasurer(storePath string) *StoreMeasurer {
	return &StoreMeasurer{
		storePath: storePath,
	}
}

func (s *StoreMeasurer) MeasureStore(logger lager.Logger) (int64, error) {
	logger = logger.Session("measuring-store", lager.Data{"storePath": s.storePath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	usage, err := s.measurePath(s.storePath)
	if err != nil {
		return 0, err
	}
	logger.Debug("store-usage", lager.Data{"bytes": usage})
	return usage, nil
}
