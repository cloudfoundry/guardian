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

	err := json.NewEncoder(os.Stdout).Encode(specs.Spec{Windows: &specs.Windows{}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode json: %s\n", err)
		os.Exit(1)
	}
}
