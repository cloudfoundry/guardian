package execrunner

func (d *WindowsExecRunner) GetProcesses() map[string]*process {
	return d.processes
}
