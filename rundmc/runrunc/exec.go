package runrunc

import (
	"io"
	"os"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/lager/v3"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

//counterfeiter:generate . UidGenerator
type UidGenerator interface {
	Generate() string
}

//counterfeiter:generate . Mkdirer
type Mkdirer interface {
	MkdirAs(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, path ...string) error
}

//counterfeiter:generate . BundleLoader
type BundleLoader interface {
	Load(log lager.Logger, handle string) (goci.Bndl, error)
}

//counterfeiter:generate . ExecRunner
type ExecRunner interface {
	Run(
		log lager.Logger, processID, sandboxHandle string,
		pio garden.ProcessIO, tty bool, procJSON io.Reader, extraCleanup func() error,
	) (garden.Process, error)
	RunPea(
		log lager.Logger, processID string, bundle goci.Bndl, sandboxHandle string,
		pio garden.ProcessIO, tty bool, procJSON io.Reader, extraCleanup func() error,
	) (garden.Process, error)
	Attach(log lager.Logger, sandboxHandle, processID string, io garden.ProcessIO) (garden.Process, error)
}

//counterfeiter:generate . ProcessBuilder
type ProcessBuilder interface {
	BuildProcess(bndl goci.Bndl, processSpec garden.ProcessSpec, user *users.ExecUser) *specs.Process
}

//counterfeiter:generate . Waiter
type Waiter interface {
	Wait() (int, error)
}

//counterfeiter:generate . Runner
type Runner interface {
	Run(log lager.Logger)
}

//counterfeiter:generate . PidGetter
type PidGetter interface {
	GetPid(log lager.Logger, containerHandle string) (int, error)
}

//counterfeiter:generate . WaitWatcher
type WaitWatcher interface { // get it??
	OnExit(log lager.Logger, process Waiter, onExit Runner)
}

type Watcher struct{}

func (w Watcher) OnExit(log lager.Logger, process Waiter, onExit Runner) {
	// #nosec G104 - ignore errors waiting for a process to exit, since that means the process is gone, and exactly what we want here.
	process.Wait()
	onExit.Run(log)
}
