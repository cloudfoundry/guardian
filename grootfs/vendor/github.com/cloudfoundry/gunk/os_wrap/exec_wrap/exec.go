package exec_wrap

import (
	"os/exec"
)

/* builds golang native exec object */
func NewExec() Exec {
	return new(_exec)
}

type _exec struct{}

func (_ *_exec) Command(name string, arg ...string) Cmd {
	return exec.Command(name, arg...)
}
