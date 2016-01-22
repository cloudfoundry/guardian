package main

import (
	"io/ioutil"
	"os"
	"strings"
)

func main() {
	args := strings.Join(os.Args, ",")
	ioutil.WriteFile(os.Args[1], []byte(args), 0700)
}
