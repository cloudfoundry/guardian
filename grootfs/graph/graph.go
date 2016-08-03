package graph

import (
	"fmt"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

const BUNDLES_DIR_NAME = "bundles"

type Graph struct {
	path string
}

func NewGraph(path string) *Graph {
	return &Graph{
		path: path,
	}
}

func (g *Graph) MakeBundle(logger lager.Logger, id string) (groot.Bundle, error) {
	logger = logger.Session("making-bundle", lager.Data{"graphPath": g.path, "id": id})
	logger.Debug("start")
	defer logger.Debug("end")

	bundle := NewBundle(path.Join(g.path, BUNDLES_DIR_NAME, id))
	if _, err := os.Stat(bundle.Path()); err == nil {
		return nil, fmt.Errorf("bundle for id `%s` already exists", id)
	}

	if err := os.MkdirAll(bundle.Path(), 0700); err != nil {
		return nil, fmt.Errorf("making bundle path: %s", err)
	}

	return bundle, nil
}

func (g *Graph) DeleteBundle(logger lager.Logger, id string) error {
	logger = logger.Session("delete-bundle", lager.Data{"graphPath": g.path, "id": id})
	logger.Debug("start")
	defer logger.Debug("end")

	bundle := NewBundle(path.Join(g.path, BUNDLES_DIR_NAME, id))
	if _, err := os.Stat(bundle.Path()); err != nil {
		return fmt.Errorf("bundle path not found: %s", err)
	}

	if err := os.RemoveAll(bundle.Path()); err != nil {
		return fmt.Errorf("deleting bundle path: %s", err)
	}

	return nil
}
