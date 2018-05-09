package runrunc

import (
	"io"
	"os"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
	"github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate counterfeiter . UidGenerator
//go:generate counterfeiter . UserLookupper
//go:generate counterfeiter . EnvDeterminer
//go:generate counterfeiter . Mkdirer
//go:generate counterfeiter . BundleLoader

type UidGenerator interface {
	Generate() string
}

type ExecUser struct {
	Uid   int
	Gid   int
	Sgids []int
	Home  string
}

type UserLookupper interface {
	Lookup(rootFsPath string, user string) (*ExecUser, error)
}

type Mkdirer interface {
	MkdirAs(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, path ...string) error
}

type LookupFunc func(rootfsPath, user string) (*ExecUser, error)

func (fn LookupFunc) Lookup(rootfsPath, user string) (*ExecUser, error) {
	return fn(rootfsPath, user)
}

type EnvDeterminer interface {
	EnvFor(bndl goci.Bndl, spec ProcessSpec) []string
}

type EnvFunc func(bndl goci.Bndl, spec ProcessSpec) []string

func (fn EnvFunc) EnvFor(bndl goci.Bndl, spec ProcessSpec) []string {
	return fn(bndl, spec)
}

type BundleLoader interface {
	Load(path string) (goci.Bndl, error)
}

//go:generate counterfeiter . ExecRunner
type ExecRunner interface {
	Run(
		log lager.Logger, processID, processPath, sandboxHandle, sandboxBundlePath string,
		containerRootHostUID, containerRootHostGID uint32, pio garden.ProcessIO, tty bool,
		procJSON io.Reader, extraCleanup func() error,
	) (garden.Process, error)
	Attach(log lager.Logger, processID string, io garden.ProcessIO, processesPath string) (garden.Process, error)
}

type PreparedSpec struct {
	specs.Process
	ContainerRootHostUID uint32
	ContainerRootHostGID uint32
}

//go:generate counterfeiter . ProcessBuilder
type ProcessBuilder interface {
	BuildProcess(bndl goci.Bndl, processSpec ProcessSpec) *PreparedSpec
}

type ProcessSpec struct {
	garden.ProcessSpec
	ContainerUID int
	ContainerGID int
}

//go:generate counterfeiter . Waiter
//go:generate counterfeiter . Runner

type Waiter interface {
	Wait() (int, error)
}

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
