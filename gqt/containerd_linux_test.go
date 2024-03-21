//go:build linux

package gqt_test

func getContainerdProcessPid(ctr, socket, containerID, processID string) string {
	processesOutput := runCtr(ctr, socket, []string{"tasks", "ps", containerID})
	return pidFromProcessesOutput(processesOutput, processID)
}
