package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		panic("network test plugin requires at least 3 arguments")
	}

	args := strings.Join(os.Args, " ")
	if err := ioutil.WriteFile(os.Args[1], []byte(args), 0700); err != nil {
		panic(err)
	}

	if strings.HasPrefix(os.Args[2], "--") {
		return
	}

	if os.Args[2] != "" {
		fmt.Println(os.Args[2])
	}
}
