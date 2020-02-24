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

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli/v2"
)

func main() {
	fakeRuntimePlugin := cli.NewApp()
	fakeRuntimePlugin.Name = "fakeRuntimePlugin"
	fakeRuntimePlugin.Usage = "I am FakeRuntimePlugin!"

	fakeRuntimePlugin.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name: "debug",
		},
		&cli.StringFlag{
			Name: "log",
		},
		&cli.StringFlag{
			Name: "log-handle",
		},
		&cli.StringFlag{
			Name: "log-format",
		},
		&cli.StringFlag{
			Name: "image-store",
		},
		&cli.StringFlag{
			Name: "root",
		},
	}

	fakeRuntimePlugin.Commands = []*cli.Command{
		&CreateCommand,
		&RunCommand,
		&DeleteCommand,
		&StateCommand,
		&EventsCommand,
		&ExecCommand,
		&ChildProcessCommand,
	}

	_ = fakeRuntimePlugin.Run(os.Args)
}

func writeArgs(action string) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("%s-args", action))
	bytes := []byte(strings.Join(os.Args, " "))
	err := ioutil.WriteFile(path, bytes, 0777)
	if err != nil {
		fail(err, "failed to write args %v to %s", bytes, path)
	}
}

func readOutput(action string) (string, bool) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("runtime-%s-output", action))
	content, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false
		}
		fail(err, "failed to read output from %s", path)
	}
	return string(content), true
}

var CreateCommand = cli.Command{
	Name: "create",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name: "no-new-keyring",
		},
		&cli.StringFlag{
			Name: "bundle",
		},
		&cli.StringFlag{
			Name: "pid-file",
		},
	},

	Action: func(ctx *cli.Context) error {
		writeArgs("create")

		path := ctx.String("pid-file")
		bytes := []byte(strconv.Itoa(os.Getppid()))
		if err := ioutil.WriteFile(path, bytes, 0777); err != nil {
			fail(err, "failed to write pid %v to file %s", bytes, path)
		}

		return nil
	},
}

var RunCommand = cli.Command{
	Name: "run",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name: "no-new-keyring",
		},
		&cli.StringFlag{
			Name: "bundle, b",
		},
		&cli.StringFlag{
			Name: "pid-file",
		},
		&cli.BoolFlag{
			Name: "detach, d",
		},
	},

	Action: func(ctx *cli.Context) error {
		writeArgs("run")

		path := ctx.String("pid-file")
		bytes := []byte(strconv.Itoa(os.Getppid()))
		if err := ioutil.WriteFile(path, bytes, 0777); err != nil {
			fail(err, "failed to write pid %v to file %s", bytes, path)
		}

		return nil
	},
}

var DeleteCommand = cli.Command{
	Name: "delete",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name: "force, f",
		},
	},

	Action: func(ctx *cli.Context) error {
		writeArgs("delete")

		return nil
	},
}

var StateCommand = cli.Command{
	Name:  "state",
	Flags: []cli.Flag{},

	Action: func(ctx *cli.Context) error {
		state := `{"pid":1234, "status":"created"}`
		if overrideState, ok := readOutput("state"); ok {
			state = overrideState
		}
		fmt.Println(state)
		return nil
	},
}

var EventsCommand = cli.Command{
	Name: "events",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name: "stats",
		},
	},

	Action: func(ctx *cli.Context) error {
		fmt.Printf("{}")
		return nil
	},
}

func copyFile(source, target string) {
	sourceFile, err := os.Open(source)
	if err != nil {
		fail(err, "failed to open file %s", source)
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fail(err, "failed to open file %s", target)
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		fail(err, "failed to copy file %s to %s", source, target)
	}
}

var ExecCommand = cli.Command{
	Name: "exec",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "process",
			Aliases: []string{"p"},
			Usage:   "path to the process.json",
		},
		&cli.BoolFlag{
			Name:    "detach",
			Aliases: []string{"d"},
			Usage:   "detach from the container's process",
		},
		&cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: "specify the file to write the process id to",
		},
	},

	Action: func(ctx *cli.Context) error {
		procSpecFilePath := filepath.Join(os.TempDir(), "exec-process-spec")
		copyFile(ctx.String("p"), procSpecFilePath)
		writeArgs("exec")

		var procSpec specs.Process
		procSpecFile, err := os.Open(procSpecFilePath)
		mustNot(err)
		defer procSpecFile.Close()
		must(json.NewDecoder(procSpecFile).Decode(&procSpec))

		exitCode := 0

		if procSpec.Args[0] == "exitcode-stdout-stderr" {
			exitCode, err = strconv.Atoi(procSpec.Args[1])
			mustNot(err)

			_, err = fmt.Fprintln(os.Stdout, procSpec.Args[2])
			mustNot(err)

			_, err = fmt.Fprintln(os.Stderr, procSpec.Args[3])
			mustNot(err)
		}

		// To satisfy dadoo's requirement that the runtime plugin fork SOMETHING
		childCmd := exec.Command(os.Args[0], "child", "--exitcode", fmt.Sprintf("%d", exitCode))
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
		&cli.IntFlag{
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
		fail(err, "unexpected error")
	}
}

var must = mustNot

func fail(err error, msg string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, msg+": %v", append(args, err)...)
	fmt.Fprintf(os.Stderr, msg+": %v", append(args, err)...)

	tmpFile, _ := os.Create("/tmp/fake-runtime-plugin-fail")
	fmt.Fprintf(tmpFile, msg+": %v", append(args, err)...)

	panic(err)
}
