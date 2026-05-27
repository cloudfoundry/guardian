package rundmc

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
)

//counterfeiter:generate . Streamer
type Streamer interface {
	StreamIn(log lager.Logger, handle string, spec garden.StreamInSpec) error
	StreamOut(log lager.Logger, handle string, spec garden.StreamOutSpec) (io.ReadCloser, error)
}

type NstarStreamer struct {
	runtime OCIRuntime
	nstar   NstarRunner
}

func NewNstarStreamer(runtime OCIRuntime, nstar NstarRunner) *NstarStreamer {
	return &NstarStreamer{runtime: runtime, nstar: nstar}
}

func (s *NstarStreamer) StreamIn(log lager.Logger, handle string, spec garden.StreamInSpec) error {
	state, err := s.runtime.State(log, handle)
	if err != nil {
		return fmt.Errorf("stream-in: pid not found for container")
	}
	if err := s.nstar.StreamIn(log, state.Pid, spec.Path, spec.User, spec.TarStream); err != nil {
		return fmt.Errorf("stream-in: nstar: %s", err)
	}
	return nil
}

func (s *NstarStreamer) StreamOut(log lager.Logger, handle string, spec garden.StreamOutSpec) (io.ReadCloser, error) {
	state, err := s.runtime.State(log, handle)
	if err != nil {
		return nil, fmt.Errorf("stream-out: pid not found for container")
	}
	stream, err := s.nstar.StreamOut(log, state.Pid, spec.Path, spec.User)
	if err != nil {
		return nil, fmt.Errorf("stream-out: nstar: %s", err)
	}
	return stream, nil
}

type ExecStreamer struct {
	runtime OCIRuntime
}

func NewExecStreamer(runtime OCIRuntime) *ExecStreamer {
	return &ExecStreamer{runtime: runtime}
}

func (s *ExecStreamer) StreamIn(log lager.Logger, handle string, spec garden.StreamInSpec) error {
	user := spec.User
	if user == "" {
		user = "root"
	}

	if err := s.execAndWait(log, handle, garden.ProcessSpec{
		Path: "/bin/mkdir",
		Args: []string{"-p", spec.Path},
		User: user,
	}, garden.ProcessIO{Stdout: io.Discard, Stderr: io.Discard}); err != nil {
		return fmt.Errorf("stream-in: mkdir: %s", err)
	}

	var stderrBuf bytes.Buffer
	if err := s.execAndWait(log, handle, garden.ProcessSpec{
		Path: "/bin/tar",
		Args: []string{"-xf", "-", "-C", spec.Path},
		User: user,
	}, garden.ProcessIO{Stdin: spec.TarStream, Stdout: io.Discard, Stderr: &stderrBuf}); err != nil {
		if stderrBuf.Len() > 0 {
			log.Error("stream-in-tar-stderr", fmt.Errorf("%s", stderrBuf.String()))
		}
		return fmt.Errorf("stream-in: tar: %s", err)
	}

	return nil
}

func (s *ExecStreamer) StreamOut(log lager.Logger, handle string, spec garden.StreamOutSpec) (io.ReadCloser, error) {
	path := spec.Path
	sourcePath := filepath.Dir(path)
	compressPath := filepath.Base(path)
	if strings.HasSuffix(path, "/") {
		sourcePath = path
		compressPath = "."
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	user := spec.User
	if user == "" {
		user = "root"
	}

	var stderrBuf bytes.Buffer
	process, err := s.runtime.Exec(log, handle, garden.ProcessSpec{
		Path: "/bin/tar",
		Args: []string{"-cf", "-", "-C", sourcePath, compressPath},
		User: user,
	}, garden.ProcessIO{Stdout: writer, Stderr: &stderrBuf})
	if err != nil {
		writer.Close()
		reader.Close()
		return nil, fmt.Errorf("stream-out: exec tar: %s", err)
	}

	go func() {
		exitCode, waitErr := process.Wait()
		if waitErr != nil || exitCode != 0 {
			log.Error("stream-out-tar-failed", fmt.Errorf("exit=%d stderr=%s", exitCode, stderrBuf.String()))
		}
		writer.Close()
	}()

	return reader, nil
}

func (s *ExecStreamer) execAndWait(log lager.Logger, handle string, spec garden.ProcessSpec, pio garden.ProcessIO) error {
	process, err := s.runtime.Exec(log, handle, spec, pio)
	if err != nil {
		return err
	}
	exitCode, err := process.Wait()
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("exited %d", exitCode)
	}
	return nil
}
