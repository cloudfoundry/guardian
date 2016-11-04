package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
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

	uid, err := exec.Command("id", "-u").CombinedOutput()
	if err != nil {
		panic(err)
	}

	gid, err := exec.Command("id", "-g").CombinedOutput()
	if err != nil {
		panic(err)
	}

	var bundlePath string
	if strings.TrimSpace(string(uid)) == "0" {
		bundlePath = filepath.Join("/tmp/store-path", imageID)
	} else {
		bundlePath = filepath.Join("/tmp/unpriv-store-path", imageID)
	}
	if action == "delete" {
		bundlePath = imageID
	}

	if action == "create" {
		rootFSPath := filepath.Join(bundlePath, "rootfs")
		if err := os.MkdirAll(rootFSPath, 0777); err != nil {
			ioutil.WriteFile("/tmp/error", []byte("greshkaaa"+string(uid)+string(gid)+err.Error()), 0755)
			panic(err)
		}
	}

	whoamiPath := filepath.Join(bundlePath, fmt.Sprintf("%s-whoami", action))
	err = ioutil.WriteFile(whoamiPath, []byte(fmt.Sprintf("%s - %s\n", strings.TrimSpace(string(uid)), strings.TrimSpace(string(gid)))), 0755)
	if err != nil {
		ioutil.WriteFile("/tmp/error", []byte("greshkaaa"+string(uid)+string(gid)+err.Error()), 0755)
		panic(err)
	}

	argsFilepath := filepath.Join(bundlePath, fmt.Sprintf("%s-args", action))
	err = ioutil.WriteFile(argsFilepath, []byte(fmt.Sprintf("%s", os.Args)), 0777)
	if err != nil {
		ioutil.WriteFile("/tmp/error", []byte("greshkaaa"+string(uid)+string(gid)+err.Error()), 0755)
		panic(err)
	}

	fmt.Printf(bundlePath)
}
