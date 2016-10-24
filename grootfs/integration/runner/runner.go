package runner

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"code.cloudfoundry.org/lager"
)

type Runner struct {
	GrootFSBin string
	StorePath  string
	DraxBin    string

	LogLevel lager.LogLevel
	LogFile  io.Writer

	Timeout time.Duration
}

func (g *Runner) WithLogFile(writer io.Writer) *Runner {
	ng := *g
	ng.LogFile = writer
	return &ng
}

func (g *Runner) wait(errChan chan error, cb func(error) (string, error)) (string, error) {
	if g.Timeout == 0 {
		runErr := <-errChan
		return cb(runErr)
	}

	select {
	case runErr := <-errChan:
		return cb(runErr)

	case <-time.After(g.Timeout):
		return "", errors.New(
			fmt.Sprintf("command took more than %f seconds to finish", g.Timeout.Seconds()),
		)
	}
}

func (g *Runner) makeCmd(subcommand string, subcommandArgs ...string) *exec.Cmd {
	args := []string{}
	args = append(args, "--log-level", g.logLevel(g.LogLevel))
	args = append(args, "--store", g.StorePath)
	if g.DraxBin != "" {
		args = append(args, "--drax-bin", g.DraxBin)
	}
	args = append(args, subcommand)
	args = append(args, subcommandArgs...)

	return exec.Command(g.GrootFSBin, args...)
}

func (g *Runner) logLevel(ll lager.LogLevel) string {
	switch ll {
	case lager.DEBUG:
		return "debug"
	case lager.INFO:
		return "info"
	case lager.FATAL:
		return "fatal"
	default:
		return "error"
	}
}
