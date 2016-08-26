package groot

import (
	"fmt"
	"net/url"

	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

//go:generate counterfeiter . Bundler
//go:generate counterfeiter . Bundle
//go:generate counterfeiter . ImagePuller

type Bundle interface {
	Path() string
	RootFSPath() string
}

type BundleSpec struct {
	VolumePath  string
	ImageConfig specsv1.Image
}

type Bundler interface {
	Exists(id string) (bool, error)
	Create(logger lager.Logger, id string, spec BundleSpec) (Bundle, error)
	Destroy(logger lager.Logger, id string) error
}

type IDMappingSpec struct {
	HostID      int
	NamespaceID int
	Size        int
}

type ImageSpec struct {
	ImageSrc    *url.URL
	UIDMappings []IDMappingSpec
	GIDMappings []IDMappingSpec
}

type ImagePuller interface {
	Pull(logger lager.Logger, spec ImageSpec) (BundleSpec, error)
}

type Groot struct {
	bundler           Bundler
	localImagePuller  ImagePuller
	remoteImagePuller ImagePuller
}

func IamGroot(bundler Bundler, localImagePuller, remoteImagePuller ImagePuller) *Groot {
	return &Groot{
		bundler:           bundler,
		localImagePuller:  localImagePuller,
		remoteImagePuller: remoteImagePuller,
	}
}

type CreateSpec struct {
	ID          string
	Image       string
	UIDMappings []IDMappingSpec
	GIDMappings []IDMappingSpec
}

func (g *Groot) Create(logger lager.Logger, spec CreateSpec) (Bundle, error) {
	logger = logger.Session("groot-creating", lager.Data{"bundleID": spec.ID, "spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	parsedURL, err := url.Parse(spec.Image)
	if err != nil {
		return nil, fmt.Errorf("parsing image url: %s", err)
	}

	ok, err := g.bundler.Exists(spec.ID)
	if err != nil {
		return nil, fmt.Errorf("checking id exists: %s", err)
	}
	if ok {
		return nil, fmt.Errorf("id already exists")
	}

	imageSpec := ImageSpec{
		ImageSrc:    parsedURL,
		UIDMappings: spec.UIDMappings,
		GIDMappings: spec.GIDMappings,
	}
	var bundleSpec BundleSpec
	if parsedURL.Scheme == "" {
		bundleSpec, err = g.localImagePuller.Pull(logger, imageSpec)
	} else {
		bundleSpec, err = g.remoteImagePuller.Pull(logger, imageSpec)
	}
	if err != nil {
		return nil, fmt.Errorf("pulling the image: %s", err)
	}

	bundle, err := g.bundler.Create(logger, spec.ID, bundleSpec)
	if err != nil {
		return nil, fmt.Errorf("making bundle: %s", err)
	}

	return bundle, nil
}

func (g *Groot) Delete(logger lager.Logger, id string) error {
	return g.bundler.Destroy(logger, id)
}
