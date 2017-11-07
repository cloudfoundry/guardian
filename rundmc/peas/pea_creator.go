package peas

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/signals"
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

//go:generate counterfeiter . Volumizer
type Volumizer interface {
	Create(log lager.Logger, spec garden.ContainerSpec) (specs.Spec, error)
	Destroy(log lager.Logger, handle string) error
}

//go:generate counterfeiter . PidGetter
type PidGetter interface {
	Pid(pidFilePath string) (int, error)
}

//go:generate counterfeiter . SignallerFactory
type SignallerFactory interface {
	NewSignaller(pidfilePath string) signals.Signaller
}

type PeaCreator struct {
	Volumizer        Volumizer
	PidGetter        PidGetter
	BundleGenerator  depot.BundleGenerator
	BundleSaver      depot.BundleSaver
	ProcessBuilder   runrunc.ProcessBuilder
	ContainerCreator ContainerCreator
	SignallerFactory SignallerFactory
}

func (p *PeaCreator) CreatePea(log lager.Logger, spec garden.ProcessSpec, procIO garden.ProcessIO, ctrHandle, ctrBundlePath string) (garden.Process, error) {
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
	if mkdirErr := os.MkdirAll(peaBundlePath, 0700); mkdirErr != nil {
		return nil, err
	}

	runtimeSpec, err := p.Volumizer.Create(log, garden.ContainerSpec{
		Handle: processID,
		Image:  spec.Image,
	})
	if err != nil {
		return errs("creating-volume", err)
	}

	cgroupPath := ctrHandle
	if spec.OverrideContainerLimits != nil {
		cgroupPath = processID
	}

	originalCtrInitPid, err := p.PidGetter.Pid(filepath.Join(ctrBundlePath, "pidfile"))
	if err != nil {
		return errs("reading-ctr-pid", err)
	}

	linuxNamespaces := map[string]string{}
	linuxNamespaces["mount"] = ""
	linuxNamespaces["network"] = fmt.Sprintf("/proc/%d/ns/net", originalCtrInitPid)
	linuxNamespaces["user"] = fmt.Sprintf("/proc/%d/ns/user", originalCtrInitPid)
	linuxNamespaces["ipc"] = fmt.Sprintf("/proc/%d/ns/ipc", originalCtrInitPid)
	linuxNamespaces["pid"] = fmt.Sprintf("/proc/%d/ns/pid", originalCtrInitPid)
	linuxNamespaces["uts"] = fmt.Sprintf("/proc/%d/ns/uts", originalCtrInitPid)

	bndl, err := p.BundleGenerator.Generate(gardener.DesiredContainerSpec{
		Handle:     processID,
		BaseConfig: runtimeSpec,
		CgroupPath: cgroupPath,
		Namespaces: linuxNamespaces,
	}, ctrBundlePath)
	if err != nil {
		return errs("generating-bundle", err)
	}

	if spec.Dir == "" {
		spec.Dir = "/"
	}
	uid, gid, err := parseUser(spec.User)
	if err != nil {
		return errs("parse-user", err)
	}

	preparedProcess := p.ProcessBuilder.BuildProcess(bndl, runrunc.ProcessSpec{
		ProcessSpec:  spec,
		ContainerUID: uid,
		ContainerGID: gid,
	})

	bndl = bndl.WithProcess(preparedProcess.Process)
	if err := p.BundleSaver.Save(bndl, peaBundlePath); err != nil {
		return errs("saving-bundle", err)
	}

	peaRunDone := make(chan error)
	go func(runcDone chan<- error) {
		runcDone <- p.ContainerCreator.Create(log, peaBundlePath, processID, procIO)
	}(peaRunDone)

	volumeDestroyer := func() {
		if err := p.Volumizer.Destroy(log, processID); err != nil {
			log.Error("destroying-volume", err)
		}
	}

	// There is coupling here: the pidfile path is hardcoded here and in
	// rundmc/runrunc/create.go.
	// This is probably fine for now, as we will be using execrunner/dadoo for
	// peas soon.
	signaller := p.SignallerFactory.NewSignaller(filepath.Join(peaBundlePath, "pidfile"))
	return &pearocess{
		id: processID, doneCh: peaRunDone,
		volumeDestroyer: volumeDestroyer, Signaller: signaller,
	}, nil
}

func parseUser(uidgid string) (int, int, error) {
	if uidgid == "" {
		return 0, 0, nil
	}

	errs := func() (int, int, error) {
		return 0, 0, fmt.Errorf("'%s' is not a valid uid:gid", uidgid)
	}

	uidGidComponents := strings.Split(uidgid, ":")
	if len(uidGidComponents) != 2 {
		return errs()
	}
	uid, err := strconv.Atoi(uidGidComponents[0])
	if err != nil {
		return errs()
	}
	gid, err := strconv.Atoi(uidGidComponents[1])
	if err != nil {
		return errs()
	}
	return uid, gid, nil
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
