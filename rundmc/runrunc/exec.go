package runrunc

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/opencontainers/specs/specs-go"
	"github.com/pivotal-golang/lager"
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
	Load(path string) (*goci.Bndl, error)
}

type Process interface {
	garden.Process
}

type ProcessTracker interface {
	Run(id string, cmd *exec.Cmd, io garden.ProcessIO, tty *garden.TTYSpec, pidFile string) (garden.Process, error)
}

type Execer struct {
	preparer *ExecPreparer
	runner   *ExecRunner
}

func NewExecer(execPreparer *ExecPreparer, runner *ExecRunner) *Execer {
	return &Execer{
		preparer: execPreparer,

		runner: runner,
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

type ExecRunner struct {
	pidGenerator       UidGenerator
	runc               RuncBinary
	tracker            ProcessTracker
	processJsonCleaner Cleaner
}

func NewExecRunner(pidGen UidGenerator, runc RuncBinary, tracker ProcessTracker, processJsonCleaner Cleaner) *ExecRunner {
	return &ExecRunner{
		pidGenerator:       pidGen,
		runc:               runc,
		tracker:            tracker,
		processJsonCleaner: processJsonCleaner,
	}
}

// runrunc saves a process.json and invokes runc exec
func (e *ExecRunner) Run(log lager.Logger, spec *specs.Process, processesPath, id string, tty *garden.TTYSpec, io garden.ProcessIO) (garden.Process, error) {
	pid := e.pidGenerator.Generate()

	log = log.Session("runrunc", lager.Data{"pid": pid})

	log.Debug("start")
	defer log.Debug("finished")

	if err := os.MkdirAll(processesPath, 0755); err != nil {
		log.Error("mk-processes-dir-failed", err)
		return nil, err
	}

	processJson, err := os.Create(path.Join(processesPath, fmt.Sprintf("%s.json", pid)))
	if err != nil {
		log.Error("create-process-json-failed", err)
		return nil, err
	}

	if err := json.NewEncoder(processJson).Encode(spec); err != nil {
		log.Error("json-encode-failed", err)
		return nil, err
	}

	pidFilePath := path.Join(processesPath, fmt.Sprintf("%s.pid", pid))
	cmd := e.runc.ExecCommand(id, processJson.Name(), pidFilePath)

	process, err := e.tracker.Run(pid, cmd, io, tty, pidFilePath)
	if err != nil {
		log.Error("run-failed", err)
		return nil, err
	}

	go e.processJsonCleaner.Clean(log, process, processJson.Name())

	return process, nil
}

type ExecPreparer struct {
	bundleLoader BundleLoader
	users        UserLookupper
	mkdirer      Mkdirer
}

func NewExecPreparer(bundleLoader BundleLoader, userlookup UserLookupper, mkdirer Mkdirer) *ExecPreparer {
	return &ExecPreparer{
		bundleLoader: bundleLoader,
		users:        userlookup,
		mkdirer:      mkdirer,
	}
}

func (r *ExecPreparer) Prepare(log lager.Logger, bundlePath string, spec garden.ProcessSpec) (*specs.Process, error) {
	log = log.Session("prepare")

	log.Info("start")
	defer log.Info("finished")

	bndl, err := r.bundleLoader.Load(bundlePath)
	if err != nil {
		log.Error("load-bundle-failed", err)
		return nil, err
	}

	rootFsPath := bndl.RootFS()
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

	return &specs.Process{
		Args: append([]string{spec.Path}, spec.Args...),
		Env:  envFor(u.containerUid, bndl, spec),
		User: specs.User{
			UID: uint32(u.containerUid),
			GID: uint32(u.containerGid),
		},
		Cwd:          cwd,
		Capabilities: bndl.Capabilities(),
		Rlimits:      toRlimits(spec.Limits),
		Terminal:     spec.TTY != nil,
	}, nil
}

type usr struct {
	hostUid, hostGid           int
	containerUid, containerGid int
	home                       string
}

func (r *ExecPreparer) lookupUser(bndl *goci.Bndl, rootfsPath, username string) (*usr, error) {
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

func (r *ExecPreparer) ensureDirExists(rootfsPath, dir string, uid, gid int) error {
	if err := r.mkdirer.MkdirAs(rootfsPath, uid, gid, 0755, false, dir); err != nil {
		return fmt.Errorf("create working directory: %s", err)
	}

	return nil
}

//go:generate counterfeiter . Waiter
type Waiter interface {
	Wait() (int, error)
}

//go:generate counterfeiter . Cleaner
type Cleaner interface {
	Clean(log lager.Logger, process Waiter, path string)
}

type ProcessJsonCleaner struct{}

func (ProcessJsonCleaner) Clean(log lager.Logger, process Waiter, path string) {
	process.Wait()
	os.Remove(path)
}
