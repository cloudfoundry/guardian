package runner

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"code.cloudfoundry.org/grootfs/groot"
)

func (g *Runner) Create(spec groot.CreateSpec) (string, error) {
	cmd := g.makeCmd("create", g.makeCreateArgs(spec)...)
	stdoutBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdoutBuffer
	if g.LogFile != nil {
		cmd.Stderr = g.LogFile
	}

	errChan := make(chan error)
	go func() {
		if err := cmd.Run(); err != nil {
			errChan <- err
		} else {
			errChan <- nil
		}
	}()

	return g.wait(errChan, func(runErr error) (string, error) {
		if runErr != nil {
			var errStr string
			stdout := strings.TrimSpace(stdoutBuffer.String())
			if stdout != "" {
				errStr = stdout
			} else {
				errStr = fmt.Sprintf("command existed with %s", runErr)
			}

			return "", errors.New(errStr)
		}

		bundlePath := strings.TrimSpace(stdoutBuffer.String())
		return bundlePath, nil
	})
}

func (g *Runner) makeCreateArgs(spec groot.CreateSpec) []string {
	args := []string{}
	for _, mapping := range spec.UIDMappings {
		args = append(args, "--uid-mapping",
			fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size),
		)
	}
	for _, mapping := range spec.GIDMappings {
		args = append(args, "--gid-mapping",
			fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size),
		)
	}
	if spec.DiskLimit > 0 {
		args = append(args, "--disk-limit-size-bytes",
			strconv.FormatInt(spec.DiskLimit, 10),
		)
		if spec.ExcludeImageFromQuota {
			args = append(args, "--exclude-image-from-quota")
		}
	}
	args = append(args, spec.Image)
	args = append(args, spec.ID)

	return args
}
