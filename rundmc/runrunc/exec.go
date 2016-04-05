package runrunc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	pidGenerator UidGenerator
	execPreparer *ExecPreparer
	runc         RuncBinary
	tracker      ProcessTracker
}

func NewExecer(runc RuncBinary, pids UidGenerator, tracker ProcessTracker, execPreparer *ExecPreparer) *Execer {
	return &Execer{
		pidGenerator: pids,
		execPreparer: execPreparer,
		runc:         runc,
		tracker:      tracker,
	}
}

// Exec a process in a bundle using 'runc exec'
func (e *Execer) Exec(log lager.Logger, bundlePath, id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("exec", lager.Data{"id": id, "path": spec.Path})

	pid := e.pidGenerator.Generate()

	log.Info("started", lager.Data{pid: "pid"})
	defer log.Info("finished")

	processJSONFilePath, pidFilePath, err := e.execPreparer.PrepareProcess(log, bundlePath, pid, spec, e.runc)
	if err != nil {
		log.Error("prepare-failed", err)
		return nil, err
	}

	cmd := e.runc.ExecCommand(id, processJSONFilePath, pidFilePath)

	process, err := e.tracker.Run(pid, cmd, io, spec.TTY, pidFilePath)
	if err != nil {
		log.Error("run-failed", err)
		return nil, err
	}

	go func() {
		process.Wait()
		if err := os.Remove(processJSONFilePath); err != nil {
			log.Error("remove-process-json-failed", err)
		}
	}()

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

func (r *ExecPreparer) PrepareProcess(log lager.Logger, bundlePath, pid string, spec garden.ProcessSpec, runc RuncBinary) (string, string, error) {
	log = log.Session("prepare")

	log.Info("start")
	defer log.Info("finished")

	processesPath := path.Join(bundlePath, "processes")
	if err := os.MkdirAll(processesPath, 0755); err != nil {
		log.Error("mk-processes-dir-failed", err)
		return "", "", err
	}

	bndl, err := r.bundleLoader.Load(bundlePath)
	if err != nil {
		log.Error("load-bundle-failed", err)
		return "", "", err
	}

	rootFsPath := bndl.RootFS()
	u, err := r.lookupUser(bndl, rootFsPath, spec.User)
	if err != nil {
		log.Error("lookup-user-failed", err)
		return "", "", err
	}

	cwd := u.home
	if spec.Dir != "" {
		cwd = spec.Dir
	}

	if err := r.ensureDirExists(rootFsPath, cwd, u.hostUid, u.hostGid); err != nil {
		log.Error("ensure-dir-failed", err)
		return "", "", err
	}

	processJsonPath, err := writeProcessJSON(log, specs.Process{
		Args: append([]string{spec.Path}, spec.Args...),
		Env:  envFor(u.containerUid, bndl, spec),
		User: specs.User{
			UID: uint32(u.containerUid),
			GID: uint32(u.containerGid),
		},
		Cwd:          cwd,
		Capabilities: bndl.Capabilities(),
		Rlimits:      toRlimits(spec.Limits),
	}, processesPath)

	if err != nil {
		log.Error("encode-process-json-failed", err)
		return "", "", err
	}

	pidFilePath := path.Join(processesPath, fmt.Sprintf("%s.pid", pid))
	return processJsonPath, pidFilePath, nil
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

func writeProcessJSON(log lager.Logger, spec specs.Process, processesPath string) (string, error) {
	tmpFile, err := ioutil.TempFile(processesPath, "guardianprocess")
	if err != nil {
		log.Error("tempfile-failed", err)
		return "", err
	}

	if err := json.NewEncoder(tmpFile).Encode(spec); err != nil {
		log.Error("encode-failed", err)
		return "", fmt.Errorf("writeProcessJSON: %s", err)
	}

	return tmpFile.Name(), nil
}
