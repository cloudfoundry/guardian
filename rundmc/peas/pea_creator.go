package peas

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/lager/v3"
	multierror "github.com/hashicorp/go-multierror"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	errorwrapper "github.com/pkg/errors"
)

var RootfsPath = filepath.Join(os.TempDir(), "pea-empty-rootfs")

//counterfeiter:generate . Volumizer
type Volumizer interface {
	Create(log lager.Logger, spec garden.ContainerSpec) (specs.Spec, error)
	Destroy(log lager.Logger, handle string) error
}

//counterfeiter:generate . PidGetter
type PidGetter interface {
	GetPid(log lager.Logger, handle string) (int, error)
}

//counterfeiter:generate . PrivilegedGetter
type PrivilegedGetter interface {
	Privileged(handle string) (bool, error)
}

//counterfeiter:generate . NetworkDepot
type NetworkDepot interface {
	SetupBindMounts(log lager.Logger, handle string, privileged bool, rootfsPath string) ([]garden.BindMount, error)
	Destroy(log lager.Logger, handle string) error
}

//counterfeiter:generate . BundleGenerator
type BundleGenerator interface {
	Generate(desiredContainerSpec spec.DesiredContainerSpec) (goci.Bndl, error)
}

type ExecRunner interface {
	RunPea(
		log lager.Logger, processID string, bundle goci.Bndl, sandboxHandle string,
		pio garden.ProcessIO, tty bool, procJSON io.Reader, extraCleanup func() error,
	) (garden.Process, error)
}

type PeaCreator struct {
	Volumizer        Volumizer
	PidGetter        PidGetter
	PrivilegedGetter PrivilegedGetter
	NetworkDepot     NetworkDepot
	BundleGenerator  BundleGenerator
	BundleSaver      depot.BundleSaver
	ProcessBuilder   runrunc.ProcessBuilder
	ExecRunner       ExecRunner
	PeaCleaner       gardener.PeaCleaner
}

func (p *PeaCreator) CreatePea(log lager.Logger, processSpec garden.ProcessSpec, procIO garden.ProcessIO, sandboxHandle string) (_ garden.Process, theErr error) {
	errs := func(action string, err error) (garden.Process, error) {
		wrappedErr := errorwrapper.Wrap(err, action)
		log.Error(action, wrappedErr)
		return nil, wrappedErr
	}

	log = log.Session("create-pea", lager.Data{})

	processID, err := generateProcessID(processSpec.ID)
	if err != nil {
		return nil, err
	}

	log.Info("creating", lager.Data{"process_id": processID})
	defer log.Info("done")

	privileged, err := p.PrivilegedGetter.Privileged(sandboxHandle)
	if err != nil {
		return errs("determining-privileged", err)
	}

	if processSpec.Dir == "" {
		processSpec.Dir = "/"
	}
	userUID, userGID, err := parseUser(processSpec.User)
	if err != nil {
		return errs("parse-user", err)
	}

	linuxNamespaces, err := p.linuxNamespaces(log, sandboxHandle, privileged)
	if err != nil {
		return errs("determining-namespaces", err)
	}

	runtimeSpec, err := p.Volumizer.Create(log, garden.ContainerSpec{
		Handle:     processID,
		Image:      processSpec.Image,
		Privileged: privileged,
	})
	if err != nil {
		return errs("creating-volume", err)
	}

	defaultBindMounts, err := p.NetworkDepot.SetupBindMounts(log, sandboxHandle, privileged, runtimeSpec.Root.Path)
	if err != nil {
		return errs("creating-bind-mount-sources", err)
	}

	if runtimeSpec.Windows == nil {
		runtimeSpec.Windows = new(specs.Windows)
	}

	runtimeSpec.Windows.Network = &specs.WindowsNetwork{
		NetworkSharedContainerName: sandboxHandle,
	}

	cgroupPath := filepath.Join(sandboxHandle, processID)
	// TODO: how to make it domain threaded?

	limits := garden.Limits{}
	if processSpec.OverrideContainerLimits != nil {
		cgroupPath = processID
		limits = garden.Limits{
			CPU:    processSpec.OverrideContainerLimits.CPU,
			Memory: processSpec.OverrideContainerLimits.Memory,
		}
	}

	bndl, genErr := p.BundleGenerator.Generate(spec.DesiredContainerSpec{
		Handle:     processID,
		BaseConfig: runtimeSpec,
		CgroupPath: cgroupPath,
		Limits:     limits,
		Namespaces: linuxNamespaces,
		BindMounts: append(processSpec.BindMounts, defaultBindMounts...),
		Privileged: privileged,
	})
	if genErr != nil {
		destroyErr := p.Volumizer.Destroy(log, processID)
		return errs("generating-bundle", multierror.Append(genErr, destroyErr))
	}

	preparedProcess := p.ProcessBuilder.BuildProcess(bndl, processSpec, &users.ExecUser{Uid: userUID, Gid: userGID})
	bndl = bndl.WithProcess(*preparedProcess)

	extraCleanup := func() error {
		return p.PeaCleaner.Clean(log, processID)
	}

	proc, runErr := p.ExecRunner.RunPea(log, processID, bndl, sandboxHandle, procIO, preparedProcess.Terminal, nil, extraCleanup)
	if runErr != nil {
		destroyErr := p.Volumizer.Destroy(log, processID)
		return nil, multierror.Append(runErr, destroyErr)
	}

	return proc, nil
}

func (p *PeaCreator) linuxNamespaces(log lager.Logger, sandboxHandle string, privileged bool) (map[string]string, error) {
	originalCtrInitPid, err := p.PidGetter.GetPid(log, sandboxHandle)
	if err != nil {
		return nil, errorwrapper.Wrap(err, "reading-ctr-pid")
	}

	linuxNamespaces := map[string]string{}
	linuxNamespaces["mount"] = ""
	linuxNamespaces["network"] = fmt.Sprintf("/proc/%d/ns/net", originalCtrInitPid)
	linuxNamespaces["ipc"] = fmt.Sprintf("/proc/%d/ns/ipc", originalCtrInitPid)
	linuxNamespaces["uts"] = fmt.Sprintf("/proc/%d/ns/uts", originalCtrInitPid)

	if !privileged {
		linuxNamespaces["user"] = fmt.Sprintf("/proc/%d/ns/user", originalCtrInitPid)
	}

	if cgroups.IsCgroup2UnifiedMode() {
		// peas have their own pid name space so that they can be killed independently
		// Otherwise, deleting one container will signal all processes in a shared pid namespace
		// https://github.com/opencontainers/runc/blob/main/libcontainer/container_linux.go#L386
		linuxNamespaces["pid"] = ""
		linuxNamespaces["cgroup"] = fmt.Sprintf("/proc/%d/ns/cgroup", originalCtrInitPid)
	} else {
		linuxNamespaces["pid"] = fmt.Sprintf("/proc/%d/ns/pid", originalCtrInitPid)
	}

	return linuxNamespaces, nil
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
