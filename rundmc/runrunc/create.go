package runrunc

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/lager"
)

type Creator struct {
	runcPath      string
	runcSubcmd    string
	commandRunner commandrunner.CommandRunner
}

func NewCreator(runcPath, runcSubcmd string, commandRunner commandrunner.CommandRunner) *Creator {
	return &Creator{
		runcPath,
		runcSubcmd,
		commandRunner,
	}
}

func (c *Creator) Create(log lager.Logger, bundlePath, id string, pio garden.ProcessIO) (theErr error) {
	logFilePath := filepath.Join(bundlePath, "create.log")
	pidFilePath := filepath.Join(bundlePath, "pidfile")

	log = log.Session("create", lager.Data{"bundle": bundlePath})
	log.Info("creating", lager.Data{
		"runc":        c.runcPath,
		"bundlePath":  bundlePath,
		"id":          id,
		"logPath":     logFilePath,
		"pidFilePath": pidFilePath,
	})
	defer log.Info("finished")

	globalArgs := []string{
		"--debug",
		"--log", logFilePath,
		"--log-format", "json",
	}
	subcmdArgs := []string{
		c.runcSubcmd,
		"--no-new-keyring",
		"--bundle", bundlePath,
		"--pid-file", pidFilePath,
		id,
	}
	cmd := exec.Command(c.runcPath, append(globalArgs, subcmdArgs...)...)
	cmd.Stdin = pio.Stdin
	cmd.Stdout = pio.Stdout
	cmd.Stderr = pio.Stderr
	err := c.commandRunner.Run(cmd)

	log.Info("completing")
	defer func() {
		theErr = processLogs(log, c.runcSubcmd, logFilePath, err)
	}()

	return
}

func processLogs(log lager.Logger, runcSubcmd string, logFilePath string, upstreamErr error) error {
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

	logging.ForwardRuncLogsToLager(log, "runc", buff)

	if upstreamErr != nil {
		return logging.WrapWithErrorFromLastLogLine(fmt.Sprintf("runc %s", runcSubcmd), upstreamErr, buff)
	}

	return nil
}
