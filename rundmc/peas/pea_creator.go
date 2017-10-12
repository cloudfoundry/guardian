package peas

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
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
	ExecPreparer     runrunc.ExecPreparer
	ContainerCreator ContainerCreator
}

func (p *PeaCreator) CreatePea(log lager.Logger, spec garden.ProcessSpec, procIO garden.ProcessIO, ctrBundlePath string) (garden.Process, error) {
	errs := func(action string, err error) (garden.Process, error) {
		wrappedErr := errorwrapper.Wrap(err, action)
		log.Error(action, wrappedErr)
		return nil, wrappedErr
	}

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

	rootfsURL, err := url.Parse(spec.Image.URI)
	if err != nil {
		return errs("parsing-image-uri", err)
	}

	if rootfsURL.Scheme != "raw" {
		return errs("expecting-raw-scheme", fmt.Errorf("expected scheme 'raw', got '%s'", rootfsURL.Scheme))
	}

	bndl, err := p.BundleGenerator.Generate(gardener.DesiredContainerSpec{
		Handle:     processID,
		Privileged: false,
		BaseConfig: specs.Spec{Root: &specs.Root{Path: rootfsURL.Path}},
	}, ctrBundlePath)
	if err != nil {
		return errs("generating-bundle", err)
	}

	if err := p.BundleSaver.Save(bndl, peaBundlePath); err != nil {
		return errs("saving-bundle", err)
	}

	preparedProcess, err := p.ExecPreparer.Prepare(log, peaBundlePath, spec)
	if err != nil {
		return errs("preparing-rootfs", err)
	}

	bndl = bndl.WithProcess(preparedProcess.Process)
	if err := p.BundleSaver.Save(bndl, peaBundlePath); err != nil {
		return errs("saving-bundle-again", err)
	}

	peaRunDone := make(chan error)
	go func(runcDone chan<- error) {
		err := p.ContainerCreator.Create(log, peaBundlePath, processID, procIO)
		if err != nil {
			wrappedErr := errorwrapper.Wrap(err, "creating-partially-shared-container")
			log.Error("creating-partially-shared-container", wrappedErr)
		}
		runcDone <- err
	}(peaRunDone)

	return &pearocess{id: processID, doneCh: peaRunDone}, nil
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
	id     string
	doneCh <-chan error
}

func (p pearocess) ID() string { return p.id }

func (p pearocess) Wait() (int, error) {
	// Exit code not yet supported for peas.
	<-p.doneCh
	return 0, nil
}

func (p pearocess) SetTTY(garden.TTYSpec) error { return nil }
func (p pearocess) Signal(garden.Signal) error  { return nil }
