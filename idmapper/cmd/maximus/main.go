package main

import (
	"fmt"

	"code.cloudfoundry.org/idmapper"
)

func main() {
	fmt.Println(uint32(idmapper.Min(idmapper.MustGetMaxValidUID(), idmapper.MustGetMaxValidGID())))
}
