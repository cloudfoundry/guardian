package groot

import (
	"fmt"
	"path"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Graph
//go:generate counterfeiter . Cloner

type Graph interface {
	MakeBundle(lager.Logger, string) (string, error)
}

type CloneSpec struct {
	FromDir, ToDir string
}

type Cloner interface {
	Clone(lager.Logger, CloneSpec) error
}

type Groot struct {
	graph  Graph
	cloner Cloner
}

func IamGroot(graph Graph, cloner Cloner) *Groot {
	return &Groot{
		graph:  graph,
		cloner: cloner,
	}
}

type CreateSpec struct {
	ID        string
	ImagePath string
}

func (g *Groot) Create(logger lager.Logger, spec CreateSpec) (string, error) {
	bundlePath, err := g.graph.MakeBundle(logger, spec.ID)
	if err != nil {
		return "", fmt.Errorf("making bundle: %s", err)
	}

	err = g.cloner.Clone(logger, CloneSpec{
		FromDir: spec.ImagePath,
		ToDir:   path.Join(bundlePath, "rootfs"),
	})
	if err != nil {
		return "", fmt.Errorf("cloning: %s", err)
	}

	return bundlePath, nil
}
