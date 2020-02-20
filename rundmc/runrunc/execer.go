package runrunc

import (
	"bytes"
	"encoding/json"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/lager"
	uuid "github.com/nu7hatch/gouuid"
)

type Execer struct {
	bundleLoader   BundleLoader
	processBuilder ProcessBuilder
	execRunner     ExecRunner
}

func NewExecer(bundleLoader BundleLoader, processBuilder ProcessBuilder, execRunner ExecRunner) *Execer {
	return &Execer{
		bundleLoader:   bundleLoader,
		processBuilder: processBuilder,
		execRunner:     execRunner,
	}
}

// Exec a process in a bundle using 'runc exec'
func (e *Execer) Exec(log lager.Logger, sandboxHandle string, spec garden.ProcessSpec, user users.ExecUser, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("exec", lager.Data{"path": spec.Path})

	log.Info("start")
	defer log.Info("finished")

	bundle, err := e.bundleLoader.Load(log, sandboxHandle)
	if err != nil {
		log.Error("load-bundle-failed", err)
		return nil, err
	}

	return e.ExecWithBndl(log, sandboxHandle, bundle, spec, user, io)
}

func (e *Execer) ExecWithBndl(log lager.Logger, sandboxHandle string, bundle goci.Bndl, spec garden.ProcessSpec, user users.ExecUser, io garden.ProcessIO) (garden.Process, error) {
	log = log.Session("exec-with-bndl", lager.Data{"path": spec.Path})
	log.Info("start")
	defer log.Info("finished")

	// TOOO: Move to containerizer
	if spec.Dir == "" {
		spec.Dir = user.Home
	}

	preparedSpec := e.processBuilder.BuildProcess(bundle, spec, user.Uid, user.Gid)

	processID := spec.ID
	if processID == "" {
		randomID, err := uuid.NewV4()
		if err != nil {
			return nil, err
		}
		processID = randomID.String()
	}

	encodedSpec, err := json.Marshal(preparedSpec)
	if err != nil {
		return nil, err // this could *almost* be a panic: a valid spec should always encode (but out of caution we'll error)
	}

	return e.execRunner.Run(
		log, processID, sandboxHandle, io, preparedSpec.Terminal, bytes.NewReader(encodedSpec), nil,
	)
}

// Attach attaches to an already running process by guid
func (e *Execer) Attach(log lager.Logger, id, processID string, io garden.ProcessIO) (garden.Process, error) {
	return e.execRunner.Attach(log, id, processID, io)
}
