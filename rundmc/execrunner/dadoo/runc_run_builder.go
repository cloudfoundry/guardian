package dadoo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func BuildRuncCommand(runtimePath, runMode, processPath, containerHandle, ttyConsoleSocket, logfilePath string) *exec.Cmd {
	runtimeArgs := []string{
		"--debug", "--log", logfilePath, "--log-format", "json",
		runMode,
		"-d",
		"--pid-file", filepath.Join(processPath, "pidfile"),
	}
	runtimeArgs = append(runtimeArgs, runmodeArgs(runMode, processPath)...)
	runtimeArgs = append(runtimeArgs, ttyArgs(runMode, ttyConsoleSocket)...)
	runtimeArgs = append(runtimeArgs, containerHandle)
	return exec.Command(runtimePath, runtimeArgs...)
}

func runmodeArgs(runMode, bundlePath string) []string {
	if runMode == "run" {
		return []string{"--no-new-keyring", "-b", bundlePath}
	}

	return []string{"-p", fmt.Sprintf("/proc/%d/fd/0", os.Getpid())}
}

func ttyArgs(runMode, ttyConsoleSocket string) []string {
	args := []string{}
	if ttyConsoleSocket == "" {
		return args
	}

	if runMode == "exec" {
		args = append(args, "--tty")
	}

	return append(args, "--console-socket", ttyConsoleSocket)
}
