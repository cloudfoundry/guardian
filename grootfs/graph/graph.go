package graph

import (
	"fmt"
	"os"
	"os/exec"
	"path"

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

func (g *Graph) MakeBundle(logger lager.Logger, imagePath, id string) (string, error) {
	logger = logger.Session("making-bundle", lager.Data{"graphPath": g.path, "imagePath": imagePath})
	logger.Debug("start")
	defer logger.Debug("end")

	if _, err := os.Stat(imagePath); err != nil {
		return "", fmt.Errorf("image path `%s` was not found: %s", imagePath, err)
	}

	bundlePath := path.Join(g.path, BUNDLES_DIR_NAME, id)
	if _, err := os.Stat(bundlePath); err == nil {
		return "", fmt.Errorf("bundle for id `%s` already exists", id)
	}

	if err := os.MkdirAll(bundlePath, 0700); err != nil {
		return "", fmt.Errorf("making bundle path: %s", err)
	}

	cmd := exec.Command("cp", "-rn", imagePath, path.Join(bundlePath, "rootfs"))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("copying the image in the bundle: %s", err)
	}

	return bundlePath, nil
}
