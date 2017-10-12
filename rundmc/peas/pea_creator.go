package peas

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/lager"
	uuid "github.com/nu7hatch/gouuid"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	errorwrapper "github.com/pkg/errors"
)

var RootfsPath = filepath.Join(os.TempDir(), "pea-empty-rootfs")

//go:generate counterfeiter . ContainerCreator
type ContainerCreator interface {
	Create(log lager.Logger, bundlePath, id string, io garden.ProcessIO) error
}

type PeaCreator struct {
	BundleGenerator  depot.BundleGenerator
	BundleSaver      depot.BundleSaver
	ContainerCreator ContainerCreator
}

func (p *PeaCreator) CreatePea(log lager.Logger, spec garden.ProcessSpec, ctrBundlePath string) (garden.Process, error) {
	log = log.Session("create-pea", lager.Data{})

	processID, err := generateProcessID(spec.ID)
	if err != nil {
		return nil, err
	}

	log.Info("creating", lager.Data{"process_id": processID})
	defer log.Info("done")

	peaBundlePath := filepath.Join(ctrBundlePath, "processes", processID)
	if err := os.MkdirAll(peaBundlePath, 0700); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(RootfsPath, 0777); err != nil {
		return nil, err
	}
	if err := os.Chmod(RootfsPath, 0777); err != nil {
		return nil, err
	}

	bndl, err := p.BundleGenerator.Generate(gardener.DesiredContainerSpec{
		Handle:     processID,
		Privileged: false,
		BaseConfig: specs.Spec{Root: &specs.Root{
			Path: RootfsPath,
		}},
	}, "")
	if err != nil {
		wrappedErr := errorwrapper.Wrap(err, "generating bundle")
		log.Error("generating bundle", wrappedErr)
		return nil, wrappedErr
	}

	if err := p.BundleSaver.Save(bndl, peaBundlePath); err != nil {
		wrappedErr := errorwrapper.Wrap(err, "saving bundle")
		log.Error("saving bundle", wrappedErr)
		return nil, wrappedErr
	}

	if err := p.ContainerCreator.Create(log, peaBundlePath, processID, garden.ProcessIO{}); err != nil {
		wrappedErr := errorwrapper.Wrap(err, "creating partially-shared container")
		log.Error("creating partially-shared container", wrappedErr)
		return nil, wrappedErr
	}

	return pearocess{id: processID}, nil
}

func generateProcessID(existingID string) (string, error) {
	if existingID != "" {
		return existingID, nil
	}
	id, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

type pearocess struct {
	id string
}

func (p pearocess) ID() string                  { return p.id }
func (p pearocess) Wait() (int, error)          { return 0, nil }
func (p pearocess) SetTTY(garden.TTYSpec) error { return nil }
func (p pearocess) Signal(garden.Signal) error  { return nil }
