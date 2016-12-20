package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
)

type Runner struct {
	GrootFSBin string
	StorePath  string
	DraxBin    string
	BtrfsBin   string
	ConfigPath string

	LogLevelSet bool
	LogLevel    lager.LogLevel
	LogFile     string
	MetronHost  net.IP
	MetronPort  uint16

	Stdout io.Writer
	Stderr io.Writer

	Timeout time.Duration
}

func (r Runner) WithBtrfsBin(btrfsBin string) Runner {
	nr := r
	nr.BtrfsBin = btrfsBin
	return nr
}

func (r Runner) WithMetronEndpoint(host net.IP, port uint16) Runner {
	nr := r
	nr.MetronHost = host
	nr.MetronPort = port
	return nr
}

func (r Runner) WithLogLevel(level lager.LogLevel) Runner {
	nr := r
	nr.LogLevel = level
	nr.LogLevelSet = true
	return nr
}

func (r Runner) WithoutLogLevel() Runner {
	nr := r
	nr.LogLevelSet = false
	return nr
}

func (r Runner) WithLogFile(path string) Runner {
	nr := r
	nr.LogFile = path
	return nr
}

func (r Runner) WithStdout(stdout io.Writer) Runner {
	nr := r
	nr.Stdout = stdout
	return nr
}

func (r Runner) WithStderr(stderr io.Writer) Runner {
	nr := r
	nr.Stderr = stderr
	return nr
}

func (r Runner) WithConfig(path string) Runner {
	nr := r
	nr.ConfigPath = path
	return nr
}

func (r Runner) WithStore(path string) Runner {
	nr := r
	nr.StorePath = path
	return nr
}

func (r Runner) RunSubcommand(subcommand string, args ...string) (string, error) {
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

func (r Runner) runCmd(cmd *exec.Cmd) error {
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

func (r Runner) makeCmd(subcommand string, args []string) *exec.Cmd {
	allArgs := []string{}
	if r.LogLevelSet {
		allArgs = append(allArgs, "--log-level", r.logLevel(r.LogLevel))
	}
	if r.LogFile != "" {
		allArgs = append(allArgs, "--log-file", r.LogFile)
	}
	if r.StorePath != "" {
		allArgs = append(allArgs, "--store", r.StorePath)
	}
	if r.DraxBin != "" {
		allArgs = append(allArgs, "--drax-bin", r.DraxBin)
	}
	if r.BtrfsBin != "" {
		allArgs = append(allArgs, "--btrfs-bin", r.BtrfsBin)
	}
	if r.MetronHost != nil && r.MetronPort != 0 {
		metronEndpoint := fmt.Sprintf("%s:%d", r.MetronHost.String(), r.MetronPort)
		allArgs = append(allArgs, "--metron-endpoint", metronEndpoint)
	}
	if r.ConfigPath != "" {
		allArgs = append(allArgs, "--config", r.ConfigPath)
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

func (r Runner) logLevel(ll lager.LogLevel) string {
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
