package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
)

type Runner struct {
	GrootFSBin string
	StorePath  string
	DraxBin    string

	LogLevelSet bool
	LogLevel    lager.LogLevel
	LogFile     string

	Stdout io.Writer
	Stderr io.Writer

	Timeout time.Duration
}

func (r *Runner) WithLogLevel(level lager.LogLevel) *Runner {
	nr := *r
	nr.LogLevel = level
	nr.LogLevelSet = true
	return &nr
}

func (r *Runner) WithoutLogLevel() *Runner {
	nr := *r
	nr.LogLevelSet = false
	return &nr
}

func (r *Runner) WithLogFile(path string) *Runner {
	nr := *r
	nr.LogFile = path
	return &nr
}

func (r *Runner) WithStdout(stdout io.Writer) *Runner {
	nr := *r
	nr.Stdout = stdout
	return &nr
}

func (r *Runner) WithStderr(stderr io.Writer) *Runner {
	nr := *r
	nr.Stderr = stderr
	return &nr
}

func (r *Runner) RunSubcommand(subcommand string, args ...string) (string, error) {
	stdoutBuffer := bytes.NewBuffer([]byte{})
	var stdout io.Writer
	if r.Stdout != nil {
		stdout = io.MultiWriter(r.Stdout, stdoutBuffer)
	} else {
		stdout = stdoutBuffer
	}
	r = r.WithStdout(stdout)

	cmd := r.makeCmd(subcommand, args)

	runErr := r.runCmd(cmd)
	if runErr != nil {
		errStr := fmt.Sprintf("command existed with %s", runErr)
		stdoutContents := strings.TrimSpace(stdoutBuffer.String())
		if stdoutContents != "" {
			errStr = stdoutContents
		}

		return "", errors.New(errStr)
	}

	return strings.TrimSpace(stdoutBuffer.String()), nil
}

func (r *Runner) runCmd(cmd *exec.Cmd) error {
	if r.Timeout == 0 {
		return cmd.Run()
	}

	errChan := make(chan error)
	go func() {
		errChan <- cmd.Run()
		close(errChan)
	}()

	select {
	case runErr := <-errChan:
		return runErr

	case <-time.After(r.Timeout):
		return errors.New(
			fmt.Sprintf("command took more than %f seconds to finish", r.Timeout.Seconds()),
		)
	}
}

func (r *Runner) makeCmd(subcommand string, args []string) *exec.Cmd {
	allArgs := []string{}
	if r.LogLevelSet {
		allArgs = append(allArgs, "--log-level", r.logLevel(r.LogLevel))
	}
	if r.LogFile != "" {
		allArgs = append(allArgs, "--log-file", r.LogFile)
	}
	allArgs = append(allArgs, "--store", r.StorePath)
	if r.DraxBin != "" {
		allArgs = append(allArgs, "--drax-bin", r.DraxBin)
	}

	allArgs = append(allArgs, subcommand)
	allArgs = append(allArgs, args...)

	cmd := exec.Command(r.GrootFSBin, allArgs...)
	if r.Stdout != nil {
		cmd.Stdout = r.Stdout
	}
	if r.Stderr != nil {
		cmd.Stderr = r.Stderr
	}

	return cmd
}

func (r *Runner) logLevel(ll lager.LogLevel) string {
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
