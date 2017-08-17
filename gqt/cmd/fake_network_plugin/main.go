package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 4 {
		panic("network test plugin requires at least 4 arguments")
	}

	argsFile, err := os.OpenFile(os.Args[1], os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	defer argsFile.Close()
	args := strings.Join(os.Args, " ")
	if _, err := fmt.Fprintln(argsFile, args); err != nil {
		panic(err)
	}

	input, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile(os.Args[2], input, 0600); err != nil {
		panic(err)
	}

	if os.Args[4] == "kill-garden-server" {
		if os.Args[6] == "down" {
			proc, err := os.FindProcess(os.Getppid())
			if err != nil {
				panic(err)
			}

			if err := proc.Kill(); err != nil {
				panic(err)
			}
		}
	}

	if strings.HasPrefix(os.Args[3], "--") {
		return
	}

	if os.Args[3] != "" {
		fmt.Println(os.Args[3])
	}
}
