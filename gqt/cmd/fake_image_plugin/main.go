package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	action := os.Args[1]
	imageID := os.Args[len(os.Args)-1]

	if imageID == "make-it-fail" {
		panic("image-plugin-exploded")
	} else if strings.Contains(imageID, "make-it-fail-on-destruction") && action == "delete" {
		panic("image-plugin-exploded-on-destruction")
	}

	uid := os.Getuid()
	gid := os.Getgid()

	var bundlePath string
	if uid == 0 {
		bundlePath = filepath.Join("/tmp/store-path", imageID)
	} else {
		bundlePath = filepath.Join("/tmp/unpriv-store-path", imageID)
	}

	if action == "create" {
		rootFSPath := filepath.Join(bundlePath, "rootfs")
		if err := os.MkdirAll(rootFSPath, 0777); err != nil {
			panic(err)
		}
	} else if action == "delete" {
		bundlePath = imageID
	}

	whoamiPath := filepath.Join(bundlePath, fmt.Sprintf("%s-whoami", action))
	err := ioutil.WriteFile(whoamiPath, []byte(fmt.Sprintf("%d - %d\n", uid, gid)), 0755)
	if err != nil {
		panic(err)
	}

	argsFilepath := filepath.Join(bundlePath, fmt.Sprintf("%s-args", action))
	err = ioutil.WriteFile(argsFilepath, []byte(strings.Join(os.Args, " ")), 0777)
	if err != nil {
		panic(err)
	}

	fmt.Printf(bundlePath)
}
