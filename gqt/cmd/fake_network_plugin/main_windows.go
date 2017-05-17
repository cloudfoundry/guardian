package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	filename := filepath.Join(os.TempDir(), timestamp)
	args := []byte(strings.Join(os.Args, ""))
	if err := ioutil.WriteFile(filename, args, 0644); err != nil {
		panic(err)
	}
}
