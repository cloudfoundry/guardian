package groot

import (
	"fmt"
	"net/url"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Graph
//go:generate counterfeiter . Bundle
//go:generate counterfeiter . Cloner

type Bundle interface {
	Path() string
	RootFSPath() string
}

type Graph interface {
	MakeBundle(logger lager.Logger, id string) (Bundle, error)
	DeleteBundle(logger lager.Logger, id string) error
}

type IDMappingSpec struct {
	HostID      int
	NamespaceID int
	Size        int
}

type CloneSpec struct {
	Image       string
	RootFSPath  string
	UIDMappings []IDMappingSpec
	GIDMappings []IDMappingSpec
}

type Cloner interface {
	Clone(logger lager.Logger, spec CloneSpec) error
}

type Groot struct {
	graph        Graph
	localCloner  Cloner
	remoteCloner Cloner
}

func IamGroot(graph Graph, localCloner, remoteCloner Cloner) *Groot {
	return &Groot{
		graph:        graph,
		localCloner:  localCloner,
		remoteCloner: remoteCloner,
	}
}

type CreateSpec struct {
	ID          string
	Image       string
	UIDMappings []IDMappingSpec
	GIDMappings []IDMappingSpec
}

func (g *Groot) Create(logger lager.Logger, spec CreateSpec) (Bundle, error) {
	parsedURL, err := url.Parse(spec.Image)
	if err != nil {
		return nil, fmt.Errorf("parsing image url: %s", err)
	}

	bundle, err := g.graph.MakeBundle(logger, spec.ID)
	if err != nil {
		return nil, fmt.Errorf("making bundle: %s", err)
	}

	cloneSpec := CloneSpec{
		Image:       spec.Image,
		RootFSPath:  bundle.RootFSPath(),
		UIDMappings: spec.UIDMappings,
		GIDMappings: spec.GIDMappings,
	}
	if parsedURL.Scheme == "" {
		err = g.localCloner.Clone(logger, cloneSpec)
	} else {
		err = g.remoteCloner.Clone(logger, cloneSpec)
	}
	if err != nil {
		if err := g.graph.DeleteBundle(logger.Session("cleaning-up-bundle"), spec.ID); err != nil {
			logger.Error("cleaning-up-bundle", err)
		}
		return nil, fmt.Errorf("cloning: %s", err)
	}

	return bundle, nil
}
