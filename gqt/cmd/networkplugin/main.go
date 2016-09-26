package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

func main() {
	if len(os.Args) < 4 {
		panic("network test plugin requires at least 4 arguments")
	}

	args := strings.Join(os.Args, " ")
	if err := ioutil.WriteFile(os.Args[1], []byte(args), 0700); err != nil {
		panic(err)
	}

	input, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile(os.Args[2], input, 0700); err != nil {
		panic(err)
	}

	if os.Args[4] == "kill-garden-server" {
		if os.Args[6] == "down" {
			cmd := exec.Command("pidof", "guardian")
			pid, err := cmd.Output()
			if err != nil {
				panic(err)
			}

			cmd = exec.Command("kill", "-9", strings.TrimSpace(string(pid)))
			if err := cmd.Run(); err != nil {
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
