package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

func main() {
	storePath := os.Args[2]
	action := os.Args[3]
	imageID := os.Args[len(os.Args)-1]

	if imageID == "make-it-fail" {
		panic("image-plugin-exploded")
	}

	err := ioutil.WriteFile(filepath.Join(storePath, fmt.Sprintf("%s-args", action)), []byte(fmt.Sprintf("%s", os.Args)), 0777)
	if err != nil {
		panic(err)
	}

	fmt.Printf(storePath)
}
