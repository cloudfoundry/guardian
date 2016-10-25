package main

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/idmapper/cmd"
)

const SUBGID_FILE_PATH = "/var/vcap/jobs/grootfs/subgid"

func main() {
	pid := os.Args[1]
	procGIDMap := fmt.Sprintf("/proc/%s/gid_map", pid)

	if err := cmd.MapIds(SUBGID_FILE_PATH, procGIDMap, os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}
