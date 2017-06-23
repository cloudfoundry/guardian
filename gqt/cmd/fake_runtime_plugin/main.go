package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/guardian/rundmc/runrunc"

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
		DeleteCommand,
		StateCommand,
		EventsCommand,
		ExecCommand,
		ChildProcessCommand,
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

var DeleteCommand = cli.Command{
	Name:  "delete",
	Flags: []cli.Flag{},

	Action: func(ctx *cli.Context) error {
		writeArgs("delete")

		return nil
	},
}

var StateCommand = cli.Command{
	Name:  "state",
	Flags: []cli.Flag{},

	Action: func(ctx *cli.Context) error {
		fmt.Printf(`{"status":"created"}`)
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
		procSpecFilePath := filepath.Join(os.TempDir(), "exec-process-spec")
		copyFile(ctx.String("p"), procSpecFilePath)
		writeArgs("exec")

		var procSpec runrunc.PreparedSpec
		procSpecFile, err := os.Open(procSpecFilePath)
		mustNot(err)
		defer procSpecFile.Close()
		must(json.NewDecoder(procSpecFile).Decode(&procSpec))

		exitCodeStr := procSpec.Args[1]
		exitCode, err := strconv.Atoi(exitCodeStr)
		mustNot(err)

		stdoutStr := procSpec.Args[2]
		_, err = fmt.Fprintln(os.Stdout, stdoutStr)
		mustNot(err)

		stderrStr := procSpec.Args[3]
		_, err = fmt.Fprintln(os.Stderr, stderrStr)
		mustNot(err)

		// To satisfy dadoo's requirement that the runtime plugin fork SOMETHING
		childCmd := exec.Command(os.Args[0], "child", "--exitcode", exitCodeStr)
		must(childCmd.Start())
		childPid := childCmd.Process.Pid
		must(ioutil.WriteFile(ctx.String("pid-file"), []byte(fmt.Sprintf("%d", childPid)), 0777))

		os.Exit(exitCode)

		return nil
	},
}

// Forked as external process by exec subcmd
var ChildProcessCommand = cli.Command{
	Name: "child",
	Flags: []cli.Flag{
		cli.IntFlag{
			Name: "exitcode",
		},
	},
	Action: func(ctx *cli.Context) error {
		os.Exit(ctx.Int("exitcode"))
		return nil
	},
}

func mustNot(err error) {
	if err != nil {
		panic(err)
	}
}

var must = mustNot
