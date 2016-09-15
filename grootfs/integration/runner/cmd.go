package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

type CreateCmd struct {
	GrootFSBin string
	StorePath  string
	Spec       groot.CreateSpec
	Timeout    time.Duration
	LogLevel   lager.LogLevel
	LogFile    io.Writer
}

func (c *CreateCmd) Run() (string, error) {
	cmd := c.makeCmd()
	stdoutBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdoutBuffer
	if c.LogFile != nil {
		cmd.Stderr = c.LogFile
	}

	errChan := make(chan error)
	go func() {
		if err := cmd.Run(); err != nil {
			errChan <- err
		} else {
			errChan <- nil
		}
	}()

	return c.wait(errChan, func(runErr error) (string, error) {
		if runErr != nil {
			errStr := fmt.Sprintf("command existed with %s", runErr)
			stdout := strings.TrimSpace(stdoutBuffer.String())
			if stdout != "" {
				errStr = errStr + fmt.Sprintf(": %s", stdout)
			}

			return "", errors.New(errStr)
		}

		bundlePath := strings.TrimSpace(stdoutBuffer.String())
		return bundlePath, nil
	})
}

func (c *CreateCmd) wait(errChan chan error, cb func(error) (string, error)) (string, error) {
	if c.Timeout == 0 {
		runErr := <-errChan
		return cb(runErr)
	}

	select {
	case runErr := <-errChan:
		return cb(runErr)

	case <-time.After(c.Timeout):
		return "", errors.New(
			fmt.Sprintf("command took more than %f seconds to finish", c.Timeout.Seconds()),
		)
	}
}

func (c *CreateCmd) makeCmd() *exec.Cmd {
	args := []string{}

	args = append(args, "--log-level", c.logLevel(c.LogLevel))
	args = append(args, "--store", c.StorePath)
	args = append(args, "create")
	for _, mapping := range c.Spec.UIDMappings {
		args = append(args, "--uid-mapping",
			fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size),
		)
	}
	for _, mapping := range c.Spec.GIDMappings {
		args = append(args, "--gid-mapping",
			fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size),
		)
	}
	if c.Spec.DiskLimit > 0 {
		args = append(args, "--disk-limit-size-bytes",
			strconv.FormatInt(c.Spec.DiskLimit, 10),
		)
		if c.Spec.ExclusiveLimit {
			args = append(args, "--exclude-image-from-quota")
		}
	}
	args = append(args, c.Spec.Image)
	args = append(args, c.Spec.ID)

	return exec.Command(c.GrootFSBin, args...)
}

func (c *CreateCmd) logLevel(ll lager.LogLevel) string {
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
