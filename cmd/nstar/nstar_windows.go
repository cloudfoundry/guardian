package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 5 {
		fmt.Printf("not enough args to nstar.exe, got: %d\n", len(os.Args))
		os.Exit(1)
	}

	tarBin := os.Args[1]
	pid := os.Args[2]
	username := os.Args[3]
	path := os.Args[4]

	var destination string
	if filepath.IsAbs(path) {
		containerPath := strings.TrimPrefix(path, filepath.VolumeName(path))
		destination = filepath.Join("c:\\", "proc", pid, "root", containerPath)
	} else {
		destination = filepath.Join("c:\\", "proc", pid, "root", "Users", username, path)
	}

	if err := os.MkdirAll(destination, 0755); err != nil {
		fmt.Printf("failed to create %s: %s\n", destination, err)
		os.Exit(1)
	}

	cmd := exec.Command(tarBin, "-xf", "-", "-C", destination)
	cmd.Stdin = os.Stdin
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("tar extract failed: %s\n", output)
		os.Exit(1)
	}
}
