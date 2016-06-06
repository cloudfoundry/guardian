package runrunc

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/pivotal-golang/lager"
)

type Starter struct {
	dadooPath     string
	runcPath      string
	commandRunner command_runner.CommandRunner
}

func NewStarter(dadooPath, runcPath string, commandRunner command_runner.CommandRunner) *Starter {
	return &Starter{
		dadooPath, runcPath, commandRunner,
	}
}

// Starts a bundle by running 'dadoo run runc' in the bundle directory
func (s *Starter) Start(log lager.Logger, bundlePath, id string, _ garden.ProcessIO) (theErr error) {
	log = log.Session("start", lager.Data{"bundle": bundlePath})

	log.Info("started")
	defer log.Info("finished")

	runcExitStatusR, runcExitStatusW, err := os.Pipe()
	if err != nil {
		return err
	}

	logR, logW, err := os.Pipe()
	if err != nil {
		return err
	}

	defer runcExitStatusR.Close()
	defer logR.Close()

	cmd := exec.Command(s.dadooPath, "run", s.runcPath, bundlePath, id)
	cmd.ExtraFiles = []*os.File{
		runcExitStatusW,
		logW,
	}

	if err := s.commandRunner.Start(cmd); err != nil {
		return fmt.Errorf("dadoo: %s", err)
	}

	runcExitStatusW.Close()
	logW.Close()

	go s.commandRunner.Wait(cmd) // avoid zombies, but we don't care about the exit status

	b := make([]byte, 1)
	if _, err := runcExitStatusR.Read(b); err != nil {
		return fmt.Errorf("dadoo: read container launch status: %s", err)
	}

	defer func() {
		theErr = processLogs(log, logR, theErr)
	}()

	if b[0] != 0 {
		return fmt.Errorf("exit status %d", b[0])
	}

	return nil
}

func processLogs(log lager.Logger, logs io.Reader, err error) error {
	buff, readErr := ioutil.ReadAll(logs)
	if readErr != nil {
		return fmt.Errorf("start: read log file: %s", readErr)
	}

	forwardRuncLogsToLager(log, buff)

	if err != nil {
		return wrapWithErrorFromRuncLog(log, err, buff)
	}

	return nil
}
