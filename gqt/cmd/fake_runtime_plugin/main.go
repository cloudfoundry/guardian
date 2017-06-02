package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/urfave/cli"
)

func main() {
	fakeRuntimePlugin := cli.NewApp()
	fakeRuntimePlugin.Name = "fakeRuntimePlugin"
	fakeRuntimePlugin.Usage = "I am FakeRuntimePlugin!"

	fakeRuntimePlugin.Flags = []cli.Flag{
		cli.BoolFlag{
			Name: "debug",
		},
		cli.StringFlag{
			Name: "log",
		},
		cli.StringFlag{
			Name: "newuidmap",
		},
		cli.StringFlag{
			Name: "newgidmap",
		},
	}

	fakeRuntimePlugin.Commands = []cli.Command{
		CreateCommand,
		StateCommand,
		EventsCommand,
		ExecCommand,
	}

	_ = fakeRuntimePlugin.Run(os.Args)
}

func writeArgs(action string) {
	err := ioutil.WriteFile(filepath.Join(os.TempDir(), fmt.Sprintf("%s-args", action)), []byte(strings.Join(os.Args, " ")), 0777)
	if err != nil {
		panic(err)
	}
}

var CreateCommand = cli.Command{
	Name: "create",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name: "no-new-keyring",
		},
		cli.StringFlag{
			Name: "bundle",
		},
		cli.StringFlag{
			Name: "pid-file",
		},
	},

	Action: func(ctx *cli.Context) error {
		writeArgs("create")

		if err := ioutil.WriteFile(ctx.String("pid-file"), []byte(strconv.Itoa(os.Getppid())), 0777); err != nil {
			panic(err)
		}

		return nil
	},
}

var StateCommand = cli.Command{
	Name:  "state",
	Flags: []cli.Flag{},

	Action: func(ctx *cli.Context) error {
		fmt.Printf("{}")
		return nil
	},
}

var EventsCommand = cli.Command{
	Name:  "events",
	Flags: []cli.Flag{},

	Action: func(ctx *cli.Context) error {
		fmt.Printf("{}")
		return nil
	},
}

func copyFile(source, target string) {
	sourceFile, err := os.Open(source)
	if err != nil {
		panic(err)
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		panic(err)
	}
}

var ExecCommand = cli.Command{
	Name: "exec",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "process, p",
			Usage: "path to the process.json",
		},
		cli.BoolFlag{
			Name:  "detach,d",
			Usage: "detach from the container's process",
		},
		cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: "specify the file to write the process id to",
		},
	},

	Action: func(ctx *cli.Context) error {
		copyFile(ctx.String("p"), filepath.Join(os.TempDir(), "exec-process-spec"))
		writeArgs("exec")
		return nil
	},
}
