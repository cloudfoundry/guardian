package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

func main() {
	if strings.Contains(os.Args[4], "CAUSE-TAR-TO-FAIL") {
		fmt.Println("tar error")
		os.Exit(1)
	}

	argsLog := os.Getenv("ARGS_LOG")
	stdinLog := os.Getenv("STDIN_LOG")

	if err := ioutil.WriteFile(argsLog, []byte(strings.Join(os.Args[1:], " ")), 0644); err != nil {
		panic(err)
	}

	stdin, err := os.OpenFile(stdinLog, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}
	defer stdin.Close()

	if _, err := io.Copy(stdin, os.Stdin); err != nil {
		panic(err)
	}
}
