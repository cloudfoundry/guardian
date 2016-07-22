package graph

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

func (c *Configurer) Ensure(logger lager.Logger, graphPath string) error {
	logger = logger.Session("ensuring-graph", lager.Data{"graphPath": graphPath})
	logger.Debug("start")
	defer logger.Debug("end")

	if info, err := os.Stat(graphPath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("graph path `%s` is not a directory", graphPath)
		}

		return nil
	}

	if err := os.Mkdir(graphPath, 0700); err != nil {
		return fmt.Errorf("making graph directory `%s`: %s", graphPath, err)
	}

	return nil
}
