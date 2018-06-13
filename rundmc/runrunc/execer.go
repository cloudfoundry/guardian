package runrunc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/idmapper"
	"code.cloudfoundry.org/lager"
)

type Execer struct {
	bundleLoader   BundleLoader
	processBuilder ProcessBuilder
	mkdirer        Mkdirer
	userLookuper   users.UserLookupper
	runner         ExecRunner
	processIDGen   UidGenerator
	pidGetter      PidGetter
}

func NewExecer(bundleLoader BundleLoader, processBuilder ProcessBuilder, mkdirer Mkdirer, userLookuper users.UserLookupper, runner ExecRunner, processIDGen UidGenerator, pidGetter PidGetter) *Execer {
	return &Execer{
		bundleLoader:   bundleLoader,
		processBuilder: processBuilder,
		mkdirer:        mkdirer,
		userLookuper:   userLookuper,
		runner:         runner,
		processIDGen:   processIDGen,
		pidGetter:      pidGetter,
	}
}

// Exec a process in a bundle using 'runc exec'
func (e *Execer) Exec(log lager.Logger, bundlePath, sandboxHandle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("exec", lager.Data{"id": sandboxHandle, "path": spec.Path})

	log.Info("start")
	defer log.Info("finished")

	ctrInitPid, err := e.pidGetter.GetPid(log, sandboxHandle)
	if err != nil {
		log.Error("read-pidfile-failed", err)
		return nil, err
	}

	rootfsPath := filepath.Join("/proc", strconv.Itoa(ctrInitPid), "root")
	user, err := e.userLookuper.Lookup(rootfsPath, spec.User)
	if err != nil {
		log.Error("user-lookup-failed", err)
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
		processID = e.processIDGen.Generate()
	}
	processPath := filepath.Join(processesPath, processID)
	if _, err := os.Stat(processPath); err == nil {
		return nil, errors.New(fmt.Sprintf("process ID '%s' already in use", processID))
	}

	if err := os.MkdirAll(processPath, 0700); err != nil {
		return nil, err
	}

	encodedSpec, err := json.Marshal(preparedSpec)
	if err != nil {
		return nil, err // this could *almost* be a panic: a valid spec should always encode (but out of caution we'll error)
	}

	return e.runner.Run(
		log, processID, processPath, sandboxHandle, bundlePath, io, preparedSpec.Terminal, bytes.NewReader(encodedSpec), nil,
	)
}

// Attach attaches to an already running process by guid
func (e *Execer) Attach(log lager.Logger, bundlePath, id, processID string, io garden.ProcessIO) (garden.Process, error) {
	processesPath := path.Join(bundlePath, "processes")
	return e.runner.Attach(log, processID, io, processesPath)
}
