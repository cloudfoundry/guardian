package runrunc

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

type Creator struct {
	runcPath      string
	runcRoot      string
	newuidmapPath string
	newgidmapPath string
	commandRunner commandrunner.CommandRunner
}

func NewCreator(runcPath, runcRoot, newuidmapPath, newgidmapPath string, commandRunner commandrunner.CommandRunner) *Creator {
	return &Creator{
		runcPath,
		runcRoot,
		newuidmapPath,
		newgidmapPath,
		commandRunner,
	}
}

func (c *Creator) Create(log lager.Logger, bundlePath, id string, _ garden.ProcessIO) (theErr error) {
	log = log.Session("create", lager.Data{"bundle": bundlePath})

	defer log.Info("finished")

	logFilePath := filepath.Join(bundlePath, "create.log")
	pidFilePath := filepath.Join(bundlePath, "pidfile")

	globalArgs := []string{
		"--debug",
		"--log", logFilePath,
		"--newuidmap", c.newuidmapPath,
		"--newgidmap", c.newgidmapPath,
	}

	createArgs := []string{
		"create",
		"--no-new-keyring",
		"--bundle", bundlePath,
		"--pid-file", pidFilePath,
		id,
	}

	if c.runcRoot != "" {
		globalArgs = append(globalArgs, []string{"--root", c.runcRoot}...)
	}

	cmd := exec.Command(c.runcPath, append(globalArgs, createArgs...)...)

	log.Info("creating", lager.Data{
		"runc":        c.runcPath,
		"runcRoot":    c.runcRoot,
		"bundlePath":  bundlePath,
		"id":          id,
		"logPath":     logFilePath,
		"pidFilePath": pidFilePath,
	})

	err := c.commandRunner.Run(cmd)

	defer func() {
		theErr = processLogs(log, logFilePath, err)
	}()

	return
}

func processLogs(log lager.Logger, logFilePath string, upstreamErr error) error {
	logReader, err := os.OpenFile(logFilePath, os.O_RDONLY, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info("create-log-does-not-exist")
			return nil
		}

		return fmt.Errorf("runc create: open log file '%s': %s", logFilePath, err)
	}
	defer logReader.Close()

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
