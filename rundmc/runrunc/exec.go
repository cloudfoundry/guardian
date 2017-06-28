package runrunc

import (
	"os"
	"os/exec"
	"path"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate counterfeiter . UidGenerator
//go:generate counterfeiter . UserLookupper
//go:generate counterfeiter . EnvDeterminer
//go:generate counterfeiter . Mkdirer
//go:generate counterfeiter . BundleLoader
//go:generate counterfeiter . ProcessTracker
//go:generate counterfeiter . Process

type UidGenerator interface {
	Generate() string
}

type UserLookupper interface {
	Lookup(rootFsPath string, user string) (*user.ExecUser, error)
}
type Mkdirer interface {
	MkdirAs(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, path ...string) error
}

type LookupFunc func(rootfsPath, user string) (*user.ExecUser, error)

func (fn LookupFunc) Lookup(rootfsPath, user string) (*user.ExecUser, error) {
	return fn(rootfsPath, user)
}

type EnvDeterminer interface {
	EnvFor(uid int, bndl goci.Bndl, spec garden.ProcessSpec) []string
}

type EnvFunc func(uid int, bndl goci.Bndl, spec garden.ProcessSpec) []string

func (fn EnvFunc) EnvFor(uid int, bndl goci.Bndl, spec garden.ProcessSpec) []string {
	return fn(uid, bndl, spec)
}

type BundleLoader interface {
	Load(path string) (goci.Bndl, error)
}

type Process interface {
	garden.Process
}

type ProcessTracker interface {
	Run(processID string, cmd *exec.Cmd, io garden.ProcessIO, tty *garden.TTYSpec, pidFile string) (garden.Process, error)
	Attach(processID string, io garden.ProcessIO, pidFilePath string) (garden.Process, error)
}

type Execer struct {
	preparer ExecPreparer
	runner   ExecRunner
}

func NewExecer(execPreparer ExecPreparer, runner ExecRunner) *Execer {
	return &Execer{
		preparer: execPreparer,
		runner:   runner,
	}
}

// Exec a process in a bundle using 'runc exec'
func (e *Execer) Exec(log lager.Logger, bundlePath, id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("exec", lager.Data{"id": id, "path": spec.Path})

	log.Info("start")
	defer log.Info("finished")

	preparedSpec, err := e.preparer.Prepare(log, bundlePath, spec)
	if err != nil {
		log.Error("prepare-failed", err)
		return nil, err
	}

	processesPath := path.Join(bundlePath, "processes")
	return e.runner.Run(log, spec.ID, preparedSpec, bundlePath, processesPath, id, spec.TTY, io)
}

// Attach attaches to an already running process by guid
func (e *Execer) Attach(log lager.Logger, bundlePath, id, processID string, io garden.ProcessIO) (garden.Process, error) {
	processesPath := path.Join(bundlePath, "processes")
	return e.runner.Attach(log, processID, io, processesPath)
}

//go:generate counterfeiter . ExecRunner
type ExecRunner interface {
	Run(log lager.Logger, passedID string, spec *PreparedSpec, bundlePath, processesPath, handle string, tty *garden.TTYSpec, io garden.ProcessIO) (garden.Process, error)
	Attach(log lager.Logger, processID string, io garden.ProcessIO, processesPath string) (garden.Process, error)
}

type PreparedSpec struct {
	specs.Process
	HostUID int
	HostGID int
}

//go:generate counterfeiter . ExecPreparer
type ExecPreparer interface {
	Prepare(log lager.Logger, bundlePath string, spec garden.ProcessSpec) (*PreparedSpec, error)
}

//go:generate counterfeiter . Waiter
//go:generate counterfeiter . Runner

type Waiter interface {
	Wait() (int, error)
}

type Runner interface {
	Run(log lager.Logger)
}

//go:generate counterfeiter . WaitWatcher

type WaitWatcher interface { // get it??
	OnExit(log lager.Logger, process Waiter, onExit Runner)
}

type Watcher struct{}

func (w Watcher) OnExit(log lager.Logger, process Waiter, onExit Runner) {
	process.Wait()
	onExit.Run(log)
}

type RemoveFiles []string

func (files RemoveFiles) Run(log lager.Logger) {
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			log.Error("cleanup-process-json-failed", err)
		}
	}
}

func intersect(l1 []string, l2 []string) (result []string) {
	for _, a := range l1 {
		for _, b := range l2 {
			if a == b {
				result = append(result, a)
			}
		}
	}

	return result
}
