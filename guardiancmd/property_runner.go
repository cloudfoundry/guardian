package guardiancmd

import (
	"fmt"
	"os"

	"github.com/cloudfoundry-incubator/guardian/properties"
	"github.com/pivotal-golang/lager"
)

type PropertyManagerRunner struct {
	manager *properties.Manager

	Logger LagerFlag
	Path   string
}

func (r *PropertyManagerRunner) GetManager() (*properties.Manager, error) {
	return r.manager, nil
}

func (r *PropertyManagerRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger, _ := r.Logger.Logger("property-manager")

	manager, err := properties.Load(r.Path)
	if err != nil {
		logger.Error("failed-to-load-properties", err, lager.Data{"propertiesPath": r.Path})
		return fmt.Errorf("load properties: %s", err)
	}

	r.manager = manager

	close(ready)

	<-signals

	if r.Path != "" {
		err := properties.Save(r.Path, manager)
		if err != nil {
			logger.Error("failed-to-save-properties", err, lager.Data{"propertiesPath": r.Path})
			// TODO: return this error also?
		}
	}
	return nil
}
