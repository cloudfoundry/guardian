package store

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/lager"
)

type Configurer struct {
}

func NewConfigurer() *Configurer {
	return &Configurer{}
}

func (c *Configurer) Ensure(logger lager.Logger, storePath string) error {
	logger = logger.Session("ensuring-store", lager.Data{"storePath": storePath})
	logger.Debug("start")
	defer logger.Debug("end")

	if info, err := os.Stat(storePath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("store path `%s` is not a directory", storePath)
		}

		return nil
	}

	if err := os.Mkdir(storePath, 0700); err != nil {
		return fmt.Errorf("making store directory `%s`: %s", storePath, err)
	}

	return nil
}
