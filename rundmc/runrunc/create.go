package runrunc

import (
	"fmt"
	"io"
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
	runcExtraArgs []string
	commandRunner commandrunner.CommandRunner
}

func NewCreator(runcPath string, runcExtraArgs []string, commandRunner commandrunner.CommandRunner) *Creator {
	return &Creator{
		runcPath,
		runcExtraArgs,
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

	args := []string{
		"--debug",
		"--log", logFilePath,
		"--log-format", "json",
	}
	args = append(args, c.runcExtraArgs...)
	args = append(args, []string{
		"run",
		"--detach",
		"--no-new-keyring",
		"--bundle", bundlePath,
		"--pid-file", pidFilePath,
		id,
	}...)

	cmd := exec.Command(c.runcPath, args...)

	if pio.Stdin != nil {
		pipeR, pipeW, err := os.Pipe()
		if err != nil {
			return err
		}
		// We must close pipes to avoid leaking file descriptors. The stdlib will
		// dup stdio before fork, but will not close the originals if they are of
		// type *os.File.
		defer pipeR.Close()
		go func() {
			io.Copy(pipeW, pio.Stdin)
			// If stdin is exhausted, we must close pipeW to avoid leaking file
			// descriptors. The user process will still be able to drain the pipe
			// buffer if it needs to.
			pipeW.Close()
		}()
		cmd.Stdin = pipeR
	}
	cmd.Stdout = pio.Stdout
	cmd.Stderr = pio.Stderr
	err := c.commandRunner.Run(cmd)

	log.Info("completing")
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

	logging.ForwardRuncLogsToLager(log, "runc", buff)

	if upstreamErr != nil {
		return logging.WrapWithErrorFromLastLogLine("runc run", upstreamErr, buff)
	}

	return nil
}
