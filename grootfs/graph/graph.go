package graph

import (
	"fmt"
	"os"
	"path"

	"code.cloudfoundry.org/lager"
)

type Graph struct {
	path string
}

func NewGraph(path string) *Graph {
	return &Graph{
		path: path,
	}
}

func (g *Graph) MakeBundle(logger lager.Logger, imagePath string) (string, error) {
	logger = logger.Session("making-bundle", lager.Data{"graphPath": g.path, "imagePath": imagePath})
	logger.Debug("start")
	defer logger.Debug("end")

	if _, err := os.Stat(imagePath); err != nil {
		return "", fmt.Errorf("image path `%s` was not found: %s", imagePath, err)
	}

	bundlePath := path.Join(g.path, "bundle")
	if err := os.MkdirAll(bundlePath, 0700); err != nil {
		return "", fmt.Errorf("making bundle path: %s", err)
	}

	if err := os.Symlink(imagePath, path.Join(bundlePath, "rootfs")); err != nil {
		return "", fmt.Errorf("making bundle rootfs: %s", err)
	}

	return bundlePath, nil
}
