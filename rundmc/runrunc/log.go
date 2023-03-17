package runrunc

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/lager/v3"
)

//go:generate counterfeiter . RuncCmdRunner

type LoggingCmd func(logFile string) *exec.Cmd

type generateLogFileFunc func() (*os.File, error)

type RuncCmdRunner interface {
	RunAndLog(log lager.Logger, cmd LoggingCmd) error
}

type logRunner struct {
	runner          commandrunner.CommandRunner
	generateLogFile generateLogFileFunc
}

func NewLogRunner(runner commandrunner.CommandRunner, logFileGenerator generateLogFileFunc) *logRunner {
	return &logRunner{runner, logFileGenerator}
}

func (l *logRunner) RunAndLog(log lager.Logger, loggingCmd LoggingCmd) (err error) {
	log = log.Session("run")

	logFile, err := l.generateLogFile()
	if err != nil {
		return err
	}
	err = l.runner.Run(loggingCmd(logFile.Name()))
	return forwardLogs(log, logFile, err)
}

func forwardLogs(log lager.Logger, logFile *os.File, err error) error {
	defer os.Remove(logFile.Name())
	defer logFile.Close()

	buff, readErr := ioutil.ReadAll(logFile)
	if readErr != nil {
		return fmt.Errorf("read log file: %s", readErr)
	}

	logging.ForwardRuncLogsToLager(log, "runc", buff)

	if err != nil {
		return logging.WrapWithErrorFromLastLogLine("runc", err, buff)
	}

	return nil
}

type LogDir string

func (dir LogDir) GenerateLogFile() (*os.File, error) {
	return ioutil.TempFile(string(dir), "")
}
