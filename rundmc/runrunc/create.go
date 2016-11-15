package runrunc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type Creator struct {
	runcPath      string
	undooPath     string
	commandRunner command_runner.CommandRunner
}

func NewCreator(runcPath, undooPath string, commandRunner command_runner.CommandRunner) *Creator {
	return &Creator{
		runcPath, undooPath, commandRunner,
	}
}

func (c *Creator) Create(log lager.Logger, bundlePath, id string, _ garden.ProcessIO) (theErr error) {
	log = log.Session("create", lager.Data{"bundle": bundlePath})

	defer log.Info("finished")

	logFilePath := filepath.Join(bundlePath, "create.log")
	pidFilePath := filepath.Join(bundlePath, "pidfile")
	configFilePath := filepath.Join(bundlePath, "config.json")

	bundleBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return err
	}

	bundle := &specs.Spec{}
	err = json.Unmarshal(bundleBytes, bundle)
	if err != nil {
		return err
	}

	layerPath := bundle.Root.Path
	mountsRoot := filepath.Dir(filepath.Dir(layerPath))
	layerToKeep := filepath.Base(layerPath)

	cmd := exec.Command(c.undooPath, "-log-file", logFilePath, mountsRoot, layerToKeep, c.runcPath, "--debug", "--log", logFilePath, "create", "--no-new-keyring", "--bundle", bundlePath, "--pid-file", pidFilePath, id)

	log.Info("creating", lager.Data{
		"undooPath":   c.undooPath,
		"mountsRoot":  mountsRoot,
		"layerToKeep": layerToKeep,
		"runc":        c.runcPath,
		"bundlePath":  bundlePath,
		"id":          id,
		"logPath":     logFilePath,
		"pidFilePath": pidFilePath,
	})

	err = c.commandRunner.Run(cmd)

	defer func() {
		theErr = processLogs(log, logFilePath, err)
	}()

	return
}

func processLogs(log lager.Logger, logFilePath string, upstreamErr error) error {
	logReader, err := os.OpenFile(logFilePath, os.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("runc create: open log file '%s': %s", logFilePath, err)
	}

	buff, readErr := ioutil.ReadAll(logReader)
	if readErr != nil {
		return fmt.Errorf("runc create: read log file: %s", readErr)
	}

	forwardRuncLogsToLager(log, buff)

	if upstreamErr != nil {
		return wrapWithErrorFromRuncLog(log, upstreamErr, buff)
	}

	return nil
}
