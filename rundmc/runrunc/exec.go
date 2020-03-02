package runrunc

import (
	"io"
	"os"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate counterfeiter . UidGenerator
type UidGenerator interface {
	Generate() string
}

//go:generate counterfeiter . Mkdirer
type Mkdirer interface {
	MkdirAs(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, path ...string) error
}

//go:generate counterfeiter . BundleLoader
type BundleLoader interface {
	Load(log lager.Logger, handle string) (goci.Bndl, error)
}

//go:generate counterfeiter . ExecRunner
type ExecRunner interface {
	Run(
		log lager.Logger, processID, sandboxHandle string,
		pio garden.ProcessIO, tty bool, procJSON io.Reader, extraCleanup func() error,
	) (garden.Process, error)
	RunPea(
		log lager.Logger, processID, sandboxHandle string,
		pio garden.ProcessIO, tty bool, procJSON io.Reader, extraCleanup func() error,
	) (garden.Process, error)
	Attach(log lager.Logger, sandboxHandle, processID string, io garden.ProcessIO) (garden.Process, error)
}

//go:generate counterfeiter . ProcessBuilder
type ProcessBuilder interface {
	BuildProcess(bndl goci.Bndl, processSpec garden.ProcessSpec, containerUID, containerGID int) *specs.Process
}

//go:generate counterfeiter . Waiter
type Waiter interface {
	Wait() (int, error)
}

//go:generate counterfeiter . Runner
type Runner interface {
	Run(log lager.Logger)
}

//go:generate counterfeiter . PidGetter
type PidGetter interface {
	GetPid(log lager.Logger, containerHandle string) (int, error)
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
