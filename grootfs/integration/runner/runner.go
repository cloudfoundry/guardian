package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"

	"code.cloudfoundry.org/lager"
)

type Runner struct {
	GrootFSBin string

	// Store path
	StorePath     string
	skipInitStore bool
	// Binaries
	TardisBin    string
	NewuidmapBin string
	NewgidmapBin string
	// Metrics
	MetronHost net.IP
	MetronPort uint16
	// Logging
	LogLevelSet        bool
	LogLevel           lager.LogLevel
	LogFile            string
	CleanLogFile       string
	LogTimestampFormat string
	// Streams
	Stdout io.Writer
	Stderr io.Writer
	// Configuration
	ConfigPath string
	// Registry
	InsecureRegistry string
	RegistryUsername string
	RegistryPassword string
	EnvVars          []string
	// Clean on Create
	CleanOnCreate   bool
	NoCleanOnCreate bool
	// Layer Checksum Validation
	SkipLayerValidation bool

	SysCredential  syscall.Credential
	StoreSizeBytes int
}

func (r Runner) StartSubcommand(subcommand string, args ...string) (*gexec.Session, error) {
	cmd := r.makeCmd(subcommand, args)
	if len(r.EnvVars) != 0 {
		cmd.Env = r.EnvVars
	} else {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "GOTRACEBACK=crash")

	if r.SysCredential.Uid != 0 {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &r.SysCredential,
		}
	}

	return gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
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

	if r.SysCredential.Uid != 0 {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &r.SysCredential,
		}
	}

	runErr := cmd.Run()
	stdoutContents := strings.TrimSpace(stdoutBuffer.String())
	if runErr != nil {
		var errStr string
		if stdoutContents != "" {
			errStr = stdoutContents
		} else {
			errStr = fmt.Sprintf("command exited with %s", runErr)
		}

		return "", errors.New(errStr)
	}

	return stdoutContents, nil
}

func (r Runner) makeCmd(subcommand string, args []string) *exec.Cmd {
	allArgs := []string{}
	if r.LogLevelSet {
		allArgs = append(allArgs, "--log-level", r.logLevel(r.LogLevel))
	}
	if r.LogFile != "" {
		allArgs = append(allArgs, "--log-file", r.LogFile)
	}
	if r.LogTimestampFormat != "" {
		allArgs = append(allArgs, "--log-timestamp-format", r.LogTimestampFormat)
	}
	if r.StorePath != "" {
		allArgs = append(allArgs, "--store", r.StorePath)
	}
	if r.TardisBin != "" {
		allArgs = append(allArgs, "--tardis-bin", r.TardisBin)
	}
	if r.NewuidmapBin != "" {
		allArgs = append(allArgs, "--newuidmap-bin", r.NewuidmapBin)
	}
	if r.NewgidmapBin != "" {
		allArgs = append(allArgs, "--newgidmap-bin", r.NewgidmapBin)
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
