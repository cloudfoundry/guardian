package runrunc

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate counterfeiter . UidGenerator
//go:generate counterfeiter . UserLookupper
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
	MkdirAs(rootfsPath string, uid, gid int, mode os.FileMode, recreate bool, path ...string) error
}

type LookupFunc func(rootfsPath, user string) (*user.ExecUser, error)

func (fn LookupFunc) Lookup(rootfsPath, user string) (*user.ExecUser, error) {
	return fn(rootfsPath, user)
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
	return e.runner.Run(log, preparedSpec, processesPath, id, spec.TTY, io)
}

// Attach attaches to an already running process by guid
func (e *Execer) Attach(log lager.Logger, bundlePath, id, processID string, io garden.ProcessIO) (garden.Process, error) {
	processesPath := path.Join(bundlePath, "processes")
	return e.runner.Attach(log, processID, io, processesPath)
}

//go:generate counterfeiter . ExecRunner
type ExecRunner interface {
	Run(log lager.Logger, spec *PreparedSpec, processesPath, handle string, tty *garden.TTYSpec, io garden.ProcessIO) (garden.Process, error)
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

type execPreparer struct {
	bundleLoader BundleLoader
	users        UserLookupper
	mkdirer      Mkdirer

	nonRootMaxCaps []string
}

func NewExecPreparer(bundleLoader BundleLoader, userlookup UserLookupper, mkdirer Mkdirer, nonRootMaxCaps []string) ExecPreparer {
	return &execPreparer{
		bundleLoader:   bundleLoader,
		users:          userlookup,
		mkdirer:        mkdirer,
		nonRootMaxCaps: nonRootMaxCaps,
	}
}

func (r *execPreparer) Prepare(log lager.Logger, bundlePath string, spec garden.ProcessSpec) (*PreparedSpec, error) {
	log = log.Session("prepare")

	log.Info("start")
	defer log.Info("finished")

	bndl, err := r.bundleLoader.Load(bundlePath)
	if err != nil {
		log.Error("load-bundle-failed", err)
		return nil, err
	}

	pidBytes, err := ioutil.ReadFile(filepath.Join(bundlePath, "pidfile"))
	if err != nil {
		log.Error("read-pidfile-failed", err)
		return nil, err
	}

	pid := string(pidBytes)
	rootFsPath := filepath.Join("/proc", pid, "root")
	u, err := r.lookupUser(bndl, rootFsPath, spec.User)
	if err != nil {
		log.Error("lookup-user-failed", err)
		return nil, err
	}

	cwd := u.home
	if spec.Dir != "" {
		cwd = spec.Dir
	}

	if err := r.ensureDirExists(rootFsPath, cwd, u.hostUid, u.hostGid); err != nil {
		log.Error("ensure-dir-failed", err)
		return nil, err
	}

	caps := bndl.Capabilities()
	if u.containerUid != 0 {
		caps = intersect(caps, r.nonRootMaxCaps)
	}

	return &PreparedSpec{
		HostUID: u.hostUid,
		HostGID: u.hostGid,
		Process: specs.Process{
			Args: append([]string{spec.Path}, spec.Args...),
			Env:  envFor(u.containerUid, bndl, spec),
			User: specs.User{
				UID: uint32(u.containerUid),
				GID: uint32(u.containerGid),
			},
			Cwd:             cwd,
			Capabilities:    caps,
			Rlimits:         toRlimits(spec.Limits),
			Terminal:        spec.TTY != nil,
			ApparmorProfile: bndl.Process().ApparmorProfile,
		},
	}, nil
}

type usr struct {
	hostUid, hostGid           int
	containerUid, containerGid int
	home                       string
}

func (r *execPreparer) lookupUser(bndl goci.Bndl, rootfsPath, username string) (*usr, error) {
	u, err := r.users.Lookup(rootfsPath, username)
	if err != nil {
		return nil, err
	}

	uid, gid := u.Uid, u.Gid
	if len(bndl.Spec.Linux.UIDMappings) > 0 {
		uid = rootfs_provider.MappingList(bndl.Spec.Linux.UIDMappings).Map(uid)
		gid = rootfs_provider.MappingList(bndl.Spec.Linux.GIDMappings).Map(gid)
	}

	return &usr{
		hostUid:      uid,
		hostGid:      gid,
		containerUid: u.Uid,
		containerGid: u.Gid,
		home:         u.Home,
	}, nil
}

func (r *execPreparer) ensureDirExists(rootfsPath, dir string, uid, gid int) error {
	if err := r.mkdirer.MkdirAs(rootfsPath, uid, gid, 0755, false, dir); err != nil {
		return fmt.Errorf("create working directory: %s", err)
	}

	return nil
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
