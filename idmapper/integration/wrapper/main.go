package main

import (
	"os"
	"os/exec"
)

func main() {
	fd := os.NewFile(3, "/proc/self/fd/3")
	buffer := make([]byte, 1)
	fd.Read(buffer)

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}
