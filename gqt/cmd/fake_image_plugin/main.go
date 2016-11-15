package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/guardian/imageplugin"
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

	var imagePath string
	if uid == 0 {
		imagePath = filepath.Join("/tmp/store-path", imageID)
	} else {
		imagePath = filepath.Join("/tmp/unpriv-store-path", imageID)
	}

	if action == "create" {
		rootFSPath := filepath.Join(imagePath, "rootfs")
		if err := os.MkdirAll(rootFSPath, 0777); err != nil {
			panic(err)
		}

		gardenDefaultRootfs := os.Getenv("GARDEN_TEST_ROOTFS")
		if err := copyFile(filepath.Join(gardenDefaultRootfs, "bin", "env"), filepath.Join(rootFSPath, "env")); err != nil {
			panic(err)
		}

		if err := setEnvVars(imagePath, []string{"BLA=BLE", "HELLO=world"}); err != nil {
			panic(err)
		}
	} else if action == "delete" {
		imagePath = imageID
	}

	whoamiPath := filepath.Join(imagePath, fmt.Sprintf("%s-whoami", action))
	err := ioutil.WriteFile(whoamiPath, []byte(fmt.Sprintf("%d - %d\n", uid, gid)), 0755)
	if err != nil {
		panic(err)
	}

	argsFilepath := filepath.Join(imagePath, fmt.Sprintf("%s-args", action))
	err = ioutil.WriteFile(argsFilepath, []byte(strings.Join(os.Args, " ")), 0777)
	if err != nil {
		panic(err)
	}

	fmt.Printf(imagePath)
}

func copyFile(srcPath, dstPath string) error {
	dirPath := filepath.Dir(dstPath)
	if err := os.MkdirAll(dirPath, 0777); err != nil {
		return err
	}

	reader, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	writer, err := os.Create(dstPath)
	if err != nil {
		reader.Close()
		return err
	}

	if _, err := io.Copy(writer, reader); err != nil {
		writer.Close()
		reader.Close()
		return err
	}

	writer.Close()
	reader.Close()

	return os.Chmod(writer.Name(), 0777)
}

func setEnvVars(imagePath string, env []string) error {
	image := imageplugin.Image{
		Config: imageplugin.ImageConfig{
			Env: env,
		},
	}
	imageJson, err := os.Create(filepath.Join(imagePath, "image.json"))
	if err != nil {
		return err
	}

	return json.NewEncoder(imageJson).Encode(image)
}
