package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
)

type Runner struct {
	GrootFSBin string

	Driver string
	// Store path
	StorePath string
	// Binaries
	DraxBin      string
	BtrfsBin     string
	NewuidmapBin string
	NewgidmapBin string
	// Metrics
	MetronHost net.IP
	MetronPort uint16
	// Logging
	LogLevelSet bool
	LogLevel    lager.LogLevel
	LogFile     string
	// Streams
	Stdout io.Writer
	Stderr io.Writer
	// Configuration
	ConfigPath string
	// Timeout
	Timeout time.Duration
	// Registry
	InsecureRegistry string
	RegistryUsername string
	RegistryPassword string
	EnvVars          []string
	// Clean on Create
	CleanOnCreate   bool
	NoCleanOnCreate bool

	SysCredential *syscall.Credential
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
	cmd.Env = r.EnvVars

	if r.SysCredential != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: r.SysCredential,
		}
	}

	runErr := r.runCmd(cmd)
	if runErr != nil {
		errStr := fmt.Sprintf("command exited with %s", runErr)
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
		cmd.Process.Kill() // Avoid data-race
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
	if r.Driver != "" {
		allArgs = append(allArgs, "--driver", r.Driver)
	}

	if r.DraxBin != "" {
		allArgs = append(allArgs, "--drax-bin", r.DraxBin)
	}
	if r.NewuidmapBin != "" {
		allArgs = append(allArgs, "--newuidmap-bin", r.NewuidmapBin)
	}
	if r.NewgidmapBin != "" {
		allArgs = append(allArgs, "--newgidmap-bin", r.NewgidmapBin)
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
