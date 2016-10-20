package runner

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

func (g *Groot) Clean() error {
	cmd := g.makeCmd("clean")
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

	_, err := g.wait(errChan, func(runErr error) (string, error) {
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

		return "", nil
	})

	return err
}
