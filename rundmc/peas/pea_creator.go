package peas

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/logging"
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

//go:generate counterfeiter . Volumizer
type Volumizer interface {
	Create(log lager.Logger, spec garden.ContainerSpec) (specs.Spec, error)
	Destroy(log lager.Logger, handle string) error
}

//go:generate counterfeiter . PidGetter
type PidGetter interface {
	Pid(pidFilePath string) (int, error)
}

type PeaCreator struct {
	Volumizer        Volumizer
	PidGetter        PidGetter
	BundleGenerator  depot.BundleGenerator
	BundleSaver      depot.BundleSaver
	ProcessBuilder   runrunc.ProcessBuilder
	ContainerCreator ContainerCreator
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
	if err := os.MkdirAll(peaBundlePath, 0700); err != nil {
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
	return &pearocess{id: processID, doneCh: peaRunDone, volumeDestroyer: volumeDestroyer}, nil
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

type pearocess struct {
	id              string
	doneCh          <-chan error
	volumeDestroyer func()
}

func (p pearocess) ID() string { return p.id }

func (p pearocess) Wait() (int, error) {
	runcRunErr := <-p.doneCh
	defer p.volumeDestroyer()
	if runcRunErr == nil {
		return 0, nil
	}
	if wrappedErr, ok := runcRunErr.(logging.WrappedError); ok {
		if exitErr, ok := wrappedErr.Underlying.(*exec.ExitError); ok {
			return exitErr.Sys().(syscall.WaitStatus).ExitStatus(), nil
		}
	}

	return -1, runcRunErr
}

func (p pearocess) SetTTY(garden.TTYSpec) error { return nil }
func (p pearocess) Signal(garden.Signal) error  { return nil }
