package runrunc

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/idmapper"
	"code.cloudfoundry.org/lager"
	uuid "github.com/nu7hatch/gouuid"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type Execer struct {
	bundleLoader   BundleLoader
	processBuilder ProcessBuilder
	mkdirer        Mkdirer
	userLookupper  users.UserLookupper
	runner         ExecRunner
	pidGetter      PidGetter
	bundleLookuper func(sandboxHandle string) (string, error)
}

func NewExecer(bundleLoader BundleLoader, processBuilder ProcessBuilder, mkdirer Mkdirer, userLookupper users.UserLookupper, runner ExecRunner, pidGetter PidGetter,
	bundleLookuper func(string) (string, error),
) *Execer {
	return &Execer{
		bundleLoader:   bundleLoader,
		processBuilder: processBuilder,
		mkdirer:        mkdirer,
		userLookupper:  userLookupper,
		runner:         runner,
		pidGetter:      pidGetter,
		bundleLookuper: bundleLookuper,
	}
}

// Exec a process in a bundle using 'runc exec'
func (e *Execer) Exec(log lager.Logger, sandboxHandle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("exec", lager.Data{"path": spec.Path})

	log.Info("start")
	defer log.Info("finished")

	ctrInitPid, err := e.pidGetter.GetPid(log, sandboxHandle)
	if err != nil {
		log.Error("read-pidfile-failed", err)
		return nil, err
	}

	rootfsPath := filepath.Join("/proc", strconv.Itoa(ctrInitPid), "root")
	user, err := e.userLookupper.Lookup(rootfsPath, spec.User)
	if err != nil {
		log.Error("user-lookup-failed", err)
		return nil, err
	}

	bundlePath, err := e.bundleLookuper(sandboxHandle)
	if err != nil {
		log.Error("bundlepath-lookup-failed", err)
		return nil, err
	}
	bundle, err := e.bundleLoader.Load(bundlePath)
	if err != nil {
		log.Error("load-bundle-failed", err)
		return nil, err
	}

	hostUID := idmapper.MappingList(bundle.Spec.Linux.UIDMappings).Map(user.Uid)
	hostGID := idmapper.MappingList(bundle.Spec.Linux.GIDMappings).Map(user.Gid)

	if spec.Dir == "" {
		spec.Dir = user.Home
	}

	err = e.mkdirer.MkdirAs(rootfsPath, hostUID, hostGID, 0755, false, spec.Dir)
	if err != nil {
		log.Error("create-workdir-failed", err)
		return nil, err
	}

	preparedSpec := e.processBuilder.BuildProcess(bundle, spec, user.Uid, user.Gid)

	processesPath := filepath.Join(bundlePath, "processes")

	processID := spec.ID
	if processID == "" {
		randomID, err := uuid.NewV4()
		if err != nil {
			return nil, err
		}
		processID = fmt.Sprintf("%s", randomID)
	}
	processPath := filepath.Join(processesPath, processID)
	if _, err := os.Stat(processPath); err == nil {
		return nil, errors.New(fmt.Sprintf("process ID '%s' already in use", processID))
	}

	if err := os.MkdirAll(processPath, 0700); err != nil {
		return nil, err
	}

	processBundle := goci.Bndl{
		Spec: specs.Spec{
			Process: preparedSpec,
		},
	}

	return e.runner.Run(
		log, processID, sandboxHandle, io, preparedSpec.Terminal, processBundle, nil,
	)
}

// Attach attaches to an already running process by guid
func (e *Execer) Attach(log lager.Logger, bundlePath, id, processID string, io garden.ProcessIO) (garden.Process, error) {
	processesPath := path.Join(bundlePath, "processes")
	return e.runner.Attach(log, processID, io, processesPath)
}
