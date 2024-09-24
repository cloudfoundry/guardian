package libcontainer

import (
	"errors"
	"io"
	"math"
	"os"

	"github.com/opencontainers/runc/libcontainer/configs"
)

var errInvalidProcess = errors.New("invalid process")

type processOperations interface {
	wait() (*os.ProcessState, error)
	signal(sig os.Signal) error
	pid() int
}

// Process specifies the configuration and IO for a process inside
// a container.
type Process struct {
	// The command to be run followed by any arguments.
	Args []string

	// Env specifies the environment variables for the process.
	Env []string

	// User will set the uid and gid of the executing process running inside the container
	// local to the container's user and group configuration.
	User string

	// AdditionalGroups specifies the gids that should be added to supplementary groups
	// in addition to those that the user belongs to.
	AdditionalGroups []string

	// Cwd will change the processes current working directory inside the container's rootfs.
	Cwd string

	// Stdin is a pointer to a reader which provides the standard input stream.
	Stdin io.Reader

	// Stdout is a pointer to a writer which receives the standard output stream.
	Stdout io.Writer

	// Stderr is a pointer to a writer which receives the standard error stream.
	Stderr io.Writer

	// ExtraFiles specifies additional open files to be inherited by the container
	ExtraFiles []*os.File

	// open handles to cloned binaries -- see dmz.CloneSelfExe for more details
	clonedExes []*os.File

	// Initial sizings for the console
	ConsoleWidth  uint16
	ConsoleHeight uint16

	// Capabilities specify the capabilities to keep when executing the process inside the container
	// All capabilities not specified will be dropped from the processes capability mask
	Capabilities *configs.Capabilities

	// AppArmorProfile specifies the profile to apply to the process and is
	// changed at the time the process is execed
	AppArmorProfile string

	// Label specifies the label to apply to the process.  It is commonly used by selinux
	Label string

	// NoNewPrivileges controls whether processes can gain additional privileges.
	NoNewPrivileges *bool

	// Rlimits specifies the resource limits, such as max open files, to set in the container
	// If Rlimits are not set, the container will inherit rlimits from the parent process
	Rlimits []configs.Rlimit

	// ConsoleSocket provides the masterfd console.
	ConsoleSocket *os.File

	// PidfdSocket provides process file descriptor of it own.
	PidfdSocket *os.File

	// Init specifies whether the process is the first process in the container.
	Init bool

	ops processOperations

	// LogLevel is a string containing a numeric representation of the current
	// log level (i.e. "4", but never "info"). It is passed on to runc init as
	// _LIBCONTAINER_LOGLEVEL environment variable.
	LogLevel string

	// SubCgroupPaths specifies sub-cgroups to run the process in.
	// Map keys are controller names, map values are paths (relative to
	// container's top-level cgroup).
	//
	// If empty, the default top-level container's cgroup is used.
	//
	// For cgroup v2, the only key allowed is "".
	SubCgroupPaths map[string]string

	Scheduler *configs.Scheduler

	IOPriority *configs.IOPriority
}

// Wait waits for the process to exit.
// Wait releases any resources associated with the Process
func (p Process) Wait() (*os.ProcessState, error) {
	if p.ops == nil {
		return nil, errInvalidProcess
	}
	return p.ops.wait()
}

// Pid returns the process ID
func (p Process) Pid() (int, error) {
	// math.MinInt32 is returned here, because it's invalid value
	// for the kill() system call.
	if p.ops == nil {
		return math.MinInt32, errInvalidProcess
	}
	return p.ops.pid(), nil
}

// Signal sends a signal to the Process.
func (p Process) Signal(sig os.Signal) error {
	if p.ops == nil {
		return errInvalidProcess
	}
	return p.ops.signal(sig)
}

// closeClonedExes cleans up any existing cloned binaries associated with the
// Process.
func (p *Process) closeClonedExes() {
	for _, exe := range p.clonedExes {
		_ = exe.Close()
	}
	p.clonedExes = nil
}

// IO holds the process's STDIO
type IO struct {
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}
