package runcontainerd

import (
	"io"
	"os"
	"regexp"

	"code.cloudfoundry.org/guardian/rundmc"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/event"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/lager"
	apievents "github.com/containerd/containerd/api/events"
	uuid "github.com/nu7hatch/gouuid"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate counterfeiter . ContainerManager
type ContainerManager interface {
	Create(log lager.Logger, containerID string, spec *specs.Spec, processIO func() (io.Reader, io.Writer, io.Writer)) error
	Delete(log lager.Logger, containerID string) error
	Exec(log lager.Logger, containerID, processID string, spec *specs.Process, processIO func() (io.Reader, io.Writer, io.Writer, bool)) error
	State(log lager.Logger, containerID string) (int, string, error)
	GetContainerPID(log lager.Logger, containerID string) (uint32, error)
	OOMEvents(log lager.Logger) <-chan *apievents.TaskOOM
	Spec(log lager.Logger, containerID string) (*specs.Spec, error)
	BundleIDs(filterLabels ...ContainerFilter) ([]string, error)
	RemoveBundle(lager.Logger, string) error
}

//go:generate counterfeiter . RuntimeStopper
type RuntimeStopper interface {
	Stop() error
}

//go:generate counterfeiter . ProcessManager
type ProcessManager interface {
	GetProcess(log lager.Logger, containerID, processID string) (BackingProcess, error)
	GetTask(log lager.Logger, id string) (BackingProcess, error)
}

//go:generate counterfeiter . ProcessBuilder
type ProcessBuilder interface {
	BuildProcess(bndl goci.Bndl, spec garden.ProcessSpec, uid, gid int) *specs.Process
}

//go:generate counterfeiter . Execer
type Execer interface {
	ExecWithBndl(log lager.Logger, id string, bndl goci.Bndl, spec garden.ProcessSpec, user users.ExecUser, io garden.ProcessIO) (garden.Process, error)
	Attach(log lager.Logger, id string, processId string, io garden.ProcessIO) (garden.Process, error)
}

//go:generate counterfeiter . Statser
type Statser interface {
	Stats(log lager.Logger, id string) (gardener.StatsContainerMetrics, error)
}

//go:generate counterfeiter . Mkdirer
type Mkdirer interface {
	MkdirAs(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, path ...string) error
}

//go:generate counterfeiter . PeaHandlesGetter
type PeaHandlesGetter interface {
	ContainerPeaHandles(log lager.Logger, sandboxHandle string) ([]string, error)
}

type ContainerFilter struct {
	Label        string
	Value        string
	ComparisonOp string
}

type RunContainerd struct {
	containerManager          ContainerManager
	processManager            ProcessManager
	processBuilder            ProcessBuilder
	execer                    Execer
	statser                   Statser
	useContainerdForProcesses bool
	cgroupManager             CgroupManager
	peaHandlesGetter          PeaHandlesGetter
	cleanupProcessDirsOnWait  bool
	runtimeStopper            RuntimeStopper
}

func New(containerManager ContainerManager,
	processManager ProcessManager,
	processBuilder ProcessBuilder,
	userLookupper users.UserLookupper,
	execer Execer,
	statser Statser,
	useContainerdForProcesses bool,
	cgroupManager CgroupManager,
	mkdirer Mkdirer,
	peaHandlesGetter PeaHandlesGetter,
	cleanupProcessDirsOnWait bool,
	runtimeStopper RuntimeStopper) *RunContainerd {
	return &RunContainerd{
		containerManager:          containerManager,
		processManager:            processManager,
		processBuilder:            processBuilder,
		execer:                    execer,
		statser:                   statser,
		useContainerdForProcesses: useContainerdForProcesses,
		cgroupManager:             cgroupManager,
		peaHandlesGetter:          peaHandlesGetter,
		cleanupProcessDirsOnWait:  cleanupProcessDirsOnWait,
		runtimeStopper:            runtimeStopper,
	}
}

func (r *RunContainerd) Create(log lager.Logger, id string, bundle goci.Bndl, pio garden.ProcessIO) error {
	log.Debug("Annotations before update", lager.Data{"id": id, "Annotations": bundle.Spec.Annotations})
	updateAnnotationsIfNeeded(&bundle)
	log.Debug("Annotations after update", lager.Data{"id": id, "Annotations": bundle.Spec.Annotations})

	err := r.containerManager.Create(log, id, &bundle.Spec, func() (io.Reader, io.Writer, io.Writer) { return pio.Stdin, pio.Stdout, pio.Stderr })
	if err != nil {
		return err
	}

	if r.useContainerdForProcesses {
		return r.cgroupManager.SetUseMemoryHierarchy(id)
	}

	return nil
}

func updateAnnotationsIfNeeded(bundle *goci.Bndl) {
	if _, ok := bundle.Spec.Annotations["container-type"]; !ok {
		if bundle.Spec.Annotations == nil {
			bundle.Spec.Annotations = make(map[string]string)
		}
		bundle.Spec.Annotations["container-type"] = "garden-init"
	}
}

func (r *RunContainerd) Exec(log lager.Logger, containerID string, gardenProcessSpec garden.ProcessSpec, user users.ExecUser, gardenIO garden.ProcessIO) (garden.Process, error) {
	bundle, err := r.getBundle(log, containerID)
	if err != nil {
		return nil, err
	}

	if !r.useContainerdForProcesses {
		return r.execer.ExecWithBndl(log, containerID, bundle, gardenProcessSpec, user, gardenIO)
	}

	// TOOO: Move to containerizer
	if gardenProcessSpec.Dir == "" {
		gardenProcessSpec.Dir = user.Home
	}

	if gardenProcessSpec.ID == "" {
		randomID, err := uuid.NewV4()
		if err != nil {
			return nil, err
		}
		gardenProcessSpec.ID = randomID.String()
	}

	processIO := func() (io.Reader, io.Writer, io.Writer, bool) {
		return gardenIO.Stdin, gardenIO.Stdout, gardenIO.Stderr, gardenProcessSpec.TTY != nil
	}

	ociProcessSpec := r.processBuilder.BuildProcess(bundle, gardenProcessSpec, user.Uid, user.Gid)
	if err = r.containerManager.Exec(log, containerID, gardenProcessSpec.ID, ociProcessSpec, processIO); err != nil {
		if isNoSuchExecutable(err) {
			return nil, garden.ExecutableNotFoundError{Message: err.Error()}
		}
		return nil, err
	}

	process, err := r.processManager.GetProcess(log, containerID, gardenProcessSpec.ID)
	if err != nil {
		return nil, err
	}

	return NewProcess(log, process, r.cleanupProcessDirsOnWait), nil
}

func isNoSuchExecutable(err error) bool {
	noSuchFile := regexp.MustCompile(`starting container process caused \"exec: .*: stat .*: no such file or directory`)
	executableNotFound := regexp.MustCompile(`starting container process caused \"exec: .*: executable file not found in \$PATH`)

	return noSuchFile.MatchString(err.Error()) || executableNotFound.MatchString(err.Error())
}

func (r *RunContainerd) getBundle(log lager.Logger, containerID string) (goci.Bndl, error) {
	spec, err := r.containerManager.Spec(log, containerID)
	if err != nil {
		return goci.Bndl{}, err
	}

	return goci.Bndl{Spec: *spec}, nil
}

func (r *RunContainerd) Attach(log lager.Logger, sandboxID, processID string, io garden.ProcessIO) (garden.Process, error) {
	if !r.useContainerdForProcesses {
		return r.execer.Attach(log, sandboxID, processID, io)
	}

	var process BackingProcess
	var err error
	if process, err = r.processManager.GetProcess(log, sandboxID, processID); err != nil {
		if isNotFound(err) {
			return nil, garden.ProcessNotFoundError{ProcessID: processID}
		}
		return nil, err
	}
	return NewProcess(log, process, r.cleanupProcessDirsOnWait), nil
}

func (r *RunContainerd) Delete(log lager.Logger, id string) error {
	return r.containerManager.Delete(log, id)
}

func (r *RunContainerd) State(log lager.Logger, id string) (rundmc.State, error) {
	pid, status, err := r.containerManager.State(log, id)
	if err != nil {
		return rundmc.State{}, err
	}

	return rundmc.State{Pid: pid, Status: rundmc.Status(status)}, nil
}

func (r *RunContainerd) Stats(log lager.Logger, id string) (gardener.StatsContainerMetrics, error) {
	return r.statser.Stats(log, id)
}

func (r *RunContainerd) Events(log lager.Logger) (<-chan event.Event, error) {
	events := make(chan event.Event)

	go func() {
		for {
			for oomEvent := range r.containerManager.OOMEvents(log) {
				events <- event.NewOOMEvent(oomEvent.ContainerID)
			}
		}
	}()

	return events, nil
}

func (r *RunContainerd) BundleInfo(log lager.Logger, handle string) (string, goci.Bndl, error) {
	containerSpec, err := r.containerManager.Spec(log, handle)
	if isNotFound(err) {
		return "", goci.Bndl{}, garden.ContainerNotFoundError{Handle: handle}
	}
	if err != nil {
		return "", goci.Bndl{}, err
	}

	return "", goci.Bndl{Spec: *containerSpec}, nil
}

func isNotFound(err error) bool {
	_, cok := err.(ContainerNotFoundError)
	_, pok := err.(ProcessNotFoundError)
	return cok || pok
}

func (r *RunContainerd) ContainerHandles() ([]string, error) {
	// We couldn't find a way to make containerd only give us the containers with no container-type label.
	// So we just get all the non-pea ones. This should be OK because even if people want to create
	// containers using containerd, but not garden, they should not use the garden namespace.
	return r.containerManager.BundleIDs(ContainerFilter{
		Label:        "container-type",
		Value:        "pea",
		ComparisonOp: "!=",
	})
}

func (r *RunContainerd) ContainerPeaHandles(log lager.Logger, sandboxHandle string) ([]string, error) {
	if r.peaHandlesGetter != nil {
		return r.peaHandlesGetter.ContainerPeaHandles(log, sandboxHandle)
	}
	return r.containerManager.BundleIDs(
		ContainerFilter{
			Label:        "container-type",
			Value:        "pea",
			ComparisonOp: "==",
		},
		ContainerFilter{
			Label:        "sandbox-container",
			Value:        sandboxHandle,
			ComparisonOp: "==",
		},
	)
}

func (r *RunContainerd) RemoveBundle(log lager.Logger, handle string) error {
	return r.containerManager.RemoveBundle(log, handle)
}

func (r *RunContainerd) Stop() error {
	return r.runtimeStopper.Stop()
}
