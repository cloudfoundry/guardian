// +build cloudfoundry

package main

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/idmapper"
)

func init() {
	currentUID := os.Getuid()
	maximusUID := idmapper.Min(idmapper.MustGetMaxValidUID(), idmapper.MustGetMaxValidGID())
	rootUID := 0

	if currentUID == rootUID || currentUID == maximusUID {
		return
	}

	fmt.Println("Tardis was compiled with the cloudfoundry build tag")
	fmt.Printf("Your user uid `%d` does not have permission to run it\n", currentUID)
	fmt.Println("aborting")
	os.Exit(1)
}
