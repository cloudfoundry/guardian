package main

import (
	"fmt"

	"code.cloudfoundry.org/idmapper"
)

func main() {
	// #nosec G115 - MustGetValidUID/GID would never return negative numbers, so this cast is safe after moving from uint32 to int
	fmt.Println(uint(idmapper.Min(idmapper.MustGetMaxValidUID(), idmapper.MustGetMaxValidGID())))
}
