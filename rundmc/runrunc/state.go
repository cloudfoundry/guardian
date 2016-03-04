package runrunc

type Status string

const RunningStatus Status = "running"

type State struct {
	Pid    int
	Status Status
}
