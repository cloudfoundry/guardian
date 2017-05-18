package systemreporter

type Report struct {
	TopProcessesByCPU    string `json:"top_processes_by_cpu"`
	TopProcessesByMemory string `json:"top_processes_by_memory"`
	Dmesg                string `json:"dmesg"`

	PidStat string `json:"pid_stat"`
	VmStat  string `json:"vm_stat"`
	MpStat  string `json:"mp_stat"`
	IoStat  string `json:"io_stat"`
}
