package runrunc

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager/v3"
)

//counterfeiter:generate . EventsWatcher
type EventsWatcher interface {
	WatchEvents(log lager.Logger, handle string) error
}

type Creator struct {
	runc          RuncBinary
	runcExtraArgs []string
	commandRunner commandrunner.CommandRunner
	eventsWatcher EventsWatcher
	depot         Depot
}

func NewCreator(
	runc RuncBinary,
	runcExtraArgs []string,
	commandRunner commandrunner.CommandRunner,
	eventsWatcher EventsWatcher,
	depot Depot,
) *Creator {
	return &Creator{
		runc,
		runcExtraArgs,
		commandRunner,
		eventsWatcher,
		depot,
	}
}

func (c *Creator) Create(log lager.Logger, id string, bundle goci.Bndl, pio garden.ProcessIO) error {
	bundlePath, err := c.depot.Create(log, id, bundle)
	if err != nil {
		log.Error("depot-create-failed", err)
		return err
	}

	logFilePath := filepath.Join(bundlePath, "create.log")
	pidFilePath := filepath.Join(bundlePath, "pidfile")

	log = log.Session("create", lager.Data{"bundle": bundlePath})
	log.Info("creating", lager.Data{
		"runc":        fmt.Sprintf("%#v", c.runc),
		"bundlePath":  bundlePath,
		"id":          id,
		"logPath":     logFilePath,
		"pidFilePath": pidFilePath,
	})
	defer log.Info("finished")

	cmd := c.runc.RunCommand(bundlePath, pidFilePath, logFilePath, id, c.runcExtraArgs)

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
	err = c.commandRunner.Run(cmd)

	log.Info("completing")
	if err := processLogs(log, logFilePath, err); err != nil {
		return err
	}

	go func() {
		if err := c.eventsWatcher.WatchEvents(log, id); err != nil {
			log.Info("event watcher error", lager.Data{"error": err})
		}
	}()

	return nil
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

	buff, readErr := io.ReadAll(logReader)
	if readErr != nil {
		return fmt.Errorf("runc create: read log file: %s", readErr)
	}

	logging.ForwardRuncLogsToLager(log, "runc", buff)

	if upstreamErr != nil {
		return logging.WrapWithErrorFromLastLogLine("runc run", upstreamErr, buff)
	}

	return nil
}
