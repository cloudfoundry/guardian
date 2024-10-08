package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

func main() {
	err := os.WriteFile("/tmp/something", []byte(fmt.Sprintf("%#v", os.Args)), 0755)
	if err != nil {
		panic(err)
	}
	socketPath, pidPath := "", ""
	for idx, s := range os.Args {
		if s == "-console-socket" || s == "--console-socket" {
			socketPath = os.Args[idx+1]
			continue
		}

		if s == "-pid-file" || s == "--pid-file" {
			pidPath = os.Args[idx+1]
			continue
		}
	}
	fmt.Println("P", socketPath, "F", pidPath)

	// long lived process in pidFile
	cmd := exec.Command("sleep", "1000")
	err = cmd.Start()
	if err != nil {
		panic(err)
	}
	go cmd.Wait()
	pid := cmd.Process.Pid
	fmt.Println("PID", pid)
	err = os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", pid)), 0755)
	if err != nil {
		panic(err)
	}
	// write dummy stuff in the socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "myDummyMaster")
	time.Sleep(time.Second * 5)
}
