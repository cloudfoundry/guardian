package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

func main() {
	storePath := os.Args[2]
	imageID := os.Args[len(os.Args)-1]

	if imageID == "make-it-fail" {
		panic("grootfs-exploded")
	}

	// Grootfs will create bundles folder if not existent
	bundlesPath := filepath.Join(storePath, "bundles")
	if _, err := os.Stat(bundlesPath); err != nil {
		if err := os.Mkdir(bundlesPath, 0777); err != nil {
			panic(err)
		}
	}

	bundlePath := filepath.Join(bundlesPath, imageID)
	if err := os.Mkdir(bundlePath, 0777); err != nil {
		panic(err)
	}

	rootfsPath := filepath.Join(bundlePath, "rootfs")
	if err := os.Mkdir(rootfsPath, 0777); err != nil {
		panic(err)
	}

	err := ioutil.WriteFile(filepath.Join(rootfsPath, "args"), []byte(fmt.Sprintf("%s", os.Args)), 0777)
	if err != nil {
		panic(err)
	}

	fmt.Printf(rootfsPath)
}
