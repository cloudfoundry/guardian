package runrunc

import (
	"io/ioutil"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/idmapper"
	"code.cloudfoundry.org/lager"
)

type Execer struct {
	bundleLoader   BundleLoader
	processBuilder ProcessBuilder
	mkdirer        Mkdirer
	userLookuper   UserLookupper
	runner         ExecRunner
}

func NewExecer(bundleLoader BundleLoader, processBuilder ProcessBuilder, mkdirer Mkdirer, userLookuper UserLookupper, runner ExecRunner) *Execer {
	return &Execer{
		bundleLoader:   bundleLoader,
		processBuilder: processBuilder,
		mkdirer:        mkdirer,
		userLookuper:   userLookuper,
		runner:         runner,
	}
}

// Exec a process in a bundle using 'runc exec'
func (e *Execer) Exec(log lager.Logger, bundlePath, id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("exec", lager.Data{"id": id, "path": spec.Path})

	log.Info("start")
	defer log.Info("finished")

	ctrInitPid, err := ioutil.ReadFile(filepath.Join(bundlePath, "pidfile"))
	if err != nil {
		log.Error("read-pidfile-failed", err)
		return nil, err
	}
	rootfsPath := filepath.Join("/proc", string(ctrInitPid), "root")
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

	preparedSpec := e.processBuilder.BuildProcess(bundle, ProcessSpec{
		ProcessSpec:  spec,
		ContainerUID: user.Uid,
		ContainerGID: user.Gid,
	})

	processesPath := filepath.Join(bundlePath, "processes")
	return e.runner.Run(log, spec.ID, preparedSpec, bundlePath, processesPath, id, spec.TTY, io)
}

// Attach attaches to an already running process by guid
func (e *Execer) Attach(log lager.Logger, bundlePath, id, processID string, io garden.ProcessIO) (garden.Process, error) {
	processesPath := path.Join(bundlePath, "processes")
	return e.runner.Attach(log, processID, io, processesPath)
}
