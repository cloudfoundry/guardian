package main

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"

	"github.com/cloudfoundry-incubator/garden-linux/iodaemon/link"
)

func main() {
	fmt.Printf("pid = %d\n", syscall.Getpid())

	extraFd := os.NewFile(3, "extrafd")
	var msg link.SignalMsg
	if err := json.NewDecoder(extraFd).Decode(&msg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	fmt.Println("Received:", msg.Signal)
}
