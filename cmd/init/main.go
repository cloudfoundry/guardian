package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	fmt.Println("Pid 1 Running")

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGTERM)

	for {
		<-signals
	}
}
