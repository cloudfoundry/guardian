package windows_command_runner

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
)

type WindowsCommandRunner struct {
	debug bool
}

type CommandNotRunningError struct {
	cmd *exec.Cmd
}

func (e CommandNotRunningError) Error() string {
	return fmt.Sprintf("command is not running: %#v", e.cmd)
}

func New(debug bool) *WindowsCommandRunner {
	return &WindowsCommandRunner{debug}
}

func (r *WindowsCommandRunner) Run(cmd *exec.Cmd) error {
	if r.debug {
		log.Printf("executing: %s\n", prettyCommand(cmd))
		r.tee(cmd)
	}

	err := r.resolve(cmd).Run()

	if r.debug {
		if err != nil {
			log.Printf("command failed (%s): %s\n", prettyCommand(cmd), err)
		} else {
			log.Printf("command succeeded (%s)\n", prettyCommand(cmd))
		}
	}

	return err
}

func (r *WindowsCommandRunner) Start(cmd *exec.Cmd) error {
	if r.debug {
		log.Printf("spawning: %s\n", prettyCommand(cmd))
		r.tee(cmd)
	}

	err := r.resolve(cmd).Start()

	if r.debug {
		if err != nil {
			log.Printf("spawning failed: %s\n", err)
		} else {
			log.Printf("spawning succeeded\n")
		}
	}

	return err
}

func (r *WindowsCommandRunner) Background(cmd *exec.Cmd) error {
	if r.debug {
		log.Printf("backgrounding: %s\n", prettyCommand(cmd))
	}

	err := r.resolve(cmd).Start()

	if r.debug {
		if err != nil {
			log.Printf("backgrounding failed: %s\n", err)
		} else {
			log.Printf("backgrounding succeeded\n")
		}
	}

	return err
}

func (r *WindowsCommandRunner) Wait(cmd *exec.Cmd) error {
	return cmd.Wait()
}

func (r *WindowsCommandRunner) Kill(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return CommandNotRunningError{cmd}
	}

	return cmd.Process.Kill()
}

func (r *WindowsCommandRunner) Signal(cmd *exec.Cmd, signal os.Signal) error {
	if cmd.Process == nil {
		return CommandNotRunningError{cmd}
	}

	return cmd.Process.Signal(signal)
}

func (r *WindowsCommandRunner) tee(cmd *exec.Cmd) {
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	} else if cmd.Stderr != nil {
		cmd.Stderr = io.MultiWriter(cmd.Stderr, os.Stderr)
	}

	if cmd.Stdout == nil {
		cmd.Stdout = os.Stderr

	} else if cmd.Stdout != nil {
		cmd.Stdout = io.MultiWriter(cmd.Stdout, os.Stderr)
	}
}

func (r *WindowsCommandRunner) resolve(cmd *exec.Cmd) *exec.Cmd {
	originalPath := cmd.Path

	path, err := exec.LookPath(cmd.Path)
	if err != nil {
		path = cmd.Path
	}

	cmd.Path = path

	cmd.Args = append([]string{originalPath}, cmd.Args...)

	return cmd
}

func prettyCommand(cmd *exec.Cmd) string {
	return fmt.Sprintf("%v %s %v", cmd.Env, cmd.Path, cmd.Args)
}
