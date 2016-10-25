package main

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/idmapper/cmd"
)

const SUBUID_FILE_PATH = "/var/vcap/jobs/grootfs/subuid"

func main() {
	pid := os.Args[1]
	procUIDMap := fmt.Sprintf("/proc/%s/uid_map", pid)

	if err := cmd.MapIds(SUBUID_FILE_PATH, procUIDMap, os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}
