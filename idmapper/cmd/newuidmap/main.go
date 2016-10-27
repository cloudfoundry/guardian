package main

import (
	"fmt"
	"os"
	"strconv"

	"code.cloudfoundry.org/idmapper/cmd"
)

func main() {
	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	procUIDMap := fmt.Sprintf("/proc/%d/uid_map", pid)
	if err := cmd.MapIds(procUIDMap); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}
