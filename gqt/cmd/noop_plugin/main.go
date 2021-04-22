package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func main() {
	if runtime.GOOS != "windows" {
		fmt.Print("{}")

		return
	}

	json.NewEncoder(os.Stdout).Encode(specs.Spec{Windows: &specs.Windows{}})
}
