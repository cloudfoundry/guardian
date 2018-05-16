package goci

import "os/exec"

// RuncBinary is the path to a runc binary.
type RuncBinary struct {
	Path string
	Root string
}

// StartCommand returns an *exec.Cmd that, when run, will execute a given bundle.
func (runc RuncBinary) StartCommand(path, id string, detach bool, log string) *exec.Cmd {
	args := []string{"--root", runc.Root, "--debug", "--log", log, "--log-format", "json", "start"}
	if detach {
		args = append(args, "-d")
	}

	args = append(args, id)

	cmd := exec.Command(runc.Path, args...)
	cmd.Dir = path
	return cmd
}

// ExecCommand returns an *exec.Cmd that, when run, will execute a process spec
// in a running container.
func (runc RuncBinary) ExecCommand(id, processJSONPath, pidFilePath string) *exec.Cmd {
	return exec.Command(
		runc.Path, []string{"--root", runc.Root, "exec", id, "--pid-file", pidFilePath, "-p", processJSONPath}...,
	)
}

// EventsCommand returns an *exec.Cmd that, when run, will retrieve events for the container
func (runc RuncBinary) EventsCommand(id string) *exec.Cmd {
	return exec.Command(runc.Path, []string{"--root", runc.Root, "events", id}...)
}

// KillCommand returns an *exec.Cmd that, when run, will signal the running
// container.
func (runc RuncBinary) KillCommand(id, signal, logFile string) *exec.Cmd {
	return exec.Command(
		runc.Path, []string{"--root", runc.Root, "--debug", "--log", logFile, "--log-format", "json", "kill", id, signal}...,
	)
}

// StateCommand returns an *exec.Cmd that, when run, will get the state of the
// container.
func (runc RuncBinary) StateCommand(id, logFile string) *exec.Cmd {
	return exec.Command(runc.Path, []string{"--root", runc.Root, "--debug", "--log", logFile, "--log-format", "json", "state", id}...)
}

// StatsCommand returns an *exec.Cmd that, when run, will get the stats of the
// container.
func (runc RuncBinary) StatsCommand(id, logFile string) *exec.Cmd {
	return exec.Command(runc.Path, []string{"--root", runc.Root, "--debug", "--log", logFile, "--log-format", "json", "events", "--stats", id}...)
}

// DeleteCommand returns an *exec.Cmd that, when run, will signal the running
// container.
func (runc RuncBinary) DeleteCommand(id string, force bool, logFile string) *exec.Cmd {
	deleteArgs := []string{"--root", runc.Root, "--debug", "--log", logFile, "--log-format", "json", "delete"}
	if force {
		deleteArgs = append(deleteArgs, "--force")
	}
	return exec.Command(runc.Path, append(deleteArgs, id)...)
}
