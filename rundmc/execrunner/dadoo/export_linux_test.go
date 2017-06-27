package dadoo

func (d *ExecRunner) GetProcesses() map[string]*process {
	return d.processes
}
