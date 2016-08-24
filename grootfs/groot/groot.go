package groot

import (
	"fmt"
	"net/url"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Bundler
//go:generate counterfeiter . Bundle
//go:generate counterfeiter . Cloner
//go:generate counterfeiter . VolumeDriver

type Bundle interface {
	Path() string
	RootFSPath() string
}

type Bundler interface {
	Bundle(id string) Bundle
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
	Bundle      Bundle
	UIDMappings []IDMappingSpec
	GIDMappings []IDMappingSpec
}

type Cloner interface {
	Clone(logger lager.Logger, spec CloneSpec) error
}

type VolumeDriver interface {
	Path(logger lager.Logger, id string) (string, error)
	Create(logger lager.Logger, parentID, id string) (string, error)
	Snapshot(logger lager.Logger, id, path string) error
	Destroy(logger lager.Logger, path string) error
}

type Groot struct {
	bundler      Bundler
	localCloner  Cloner
	remoteCloner Cloner
	volumeDriver VolumeDriver
}

func IamGroot(bundler Bundler, localCloner, remoteCloner Cloner, volumeDriver VolumeDriver) *Groot {
	return &Groot{
		bundler:      bundler,
		localCloner:  localCloner,
		remoteCloner: remoteCloner,
		volumeDriver: volumeDriver,
	}
}

type CreateSpec struct {
	ID          string
	Image       string
	UIDMappings []IDMappingSpec
	GIDMappings []IDMappingSpec
}

func (g *Groot) Create(logger lager.Logger, spec CreateSpec) (Bundle, error) {
	logger = logger.Session("groot-creating", lager.Data{"spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	parsedURL, err := url.Parse(spec.Image)
	if err != nil {
		return nil, fmt.Errorf("parsing image url: %s", err)
	}

	bundle, err := g.bundler.MakeBundle(logger, spec.ID)
	if err != nil {
		return nil, fmt.Errorf("making bundle: %s", err)
	}

	cloneSpec := CloneSpec{
		Image:       spec.Image,
		Bundle:      bundle,
		UIDMappings: spec.UIDMappings,
		GIDMappings: spec.GIDMappings,
	}
	if parsedURL.Scheme == "" {
		err = g.localCloner.Clone(logger, cloneSpec)
	} else {
		err = g.remoteCloner.Clone(logger, cloneSpec)
	}
	if err != nil {
		if err := g.bundler.DeleteBundle(logger.Session("cleaning-up-bundle"), spec.ID); err != nil {
			logger.Error("cleaning-up-bundle", err)
		}
		return nil, fmt.Errorf("cloning: %s", err)
	}

	return bundle, nil
}

func (g *Groot) Delete(logger lager.Logger, id string) error {
	bundle := g.bundler.Bundle(id)

	if err := g.volumeDriver.Destroy(logger, bundle.RootFSPath()); err != nil {
		return err
	}

	if err := g.bundler.DeleteBundle(logger, id); err != nil {
		return fmt.Errorf("deleting bundle: %s", err)
	}

	return nil
}
