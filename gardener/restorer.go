package gardener

import "github.com/pivotal-golang/lager"

type restorer struct {
	networker Networker
}

func NewRestorer(networker Networker) Restorer {
	return &restorer{
		networker: networker,
	}
}

func (r *restorer) Restore(logger lager.Logger, handles []string) []string {
	failedHandles := []string{}

	for _, handle := range handles {
		logger.Info("restoring-container", lager.Data{"handle": handle})
		err := r.networker.Restore(logger, handle)
		if err != nil {
			logger.Error("failed-restoring-container", err, lager.Data{"handle": handle})
			failedHandles = append(failedHandles, handle)
		}
	}

	return failedHandles
}
