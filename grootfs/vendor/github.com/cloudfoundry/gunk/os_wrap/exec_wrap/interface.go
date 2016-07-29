/*
Package exec_wrap wraps golang exec in an interface.

With this you can mock exec system calls. Please add to this interface
and implementations with other calls that are to be mocked.

The fake/mock implementation is in an aptly named subdirectory.
*/
package exec_wrap

import (
	"io"
)

//go:generate counterfeiter -o execfakes/fake_cmd.go . Cmd

/*
Wraps cmd calls.
*/
type Cmd interface {
	Start() error
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Wait() error
}

//go:generate counterfeiter -o execfakes/fake_exec.go . Exec

/*
Wraps http client side calls.
*/
type Exec interface {
	Command(name string, arg ...string) Cmd
}
