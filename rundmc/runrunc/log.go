package runrunc

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner"
)

//go:generate counterfeiter . RuncCmdRunner

type LoggingCmd func(logFile string) *exec.Cmd

type generateLogFileFunc func() (*os.File, error)

type RuncCmdRunner interface {
	RunAndLog(log lager.Logger, cmd LoggingCmd) error
}

type logRunner struct {
	runner          command_runner.CommandRunner
	generateLogFile generateLogFileFunc
}

func NewLogRunner(runner command_runner.CommandRunner, logFileGenerator generateLogFileFunc) *logRunner {
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
	buff, readErr := ioutil.ReadAll(logFile)
	if readErr != nil {
		return fmt.Errorf("read log file: %s", readErr)
	}

	forwardRuncLogsToLager(log, buff)

	if err != nil {
		return wrapWithErrorFromRuncLog(log, err, buff)
	}

	return nil
}

type LogDir string

func (dir LogDir) GenerateLogFile() (*os.File, error) {
	return ioutil.TempFile(string(dir), "")
}
