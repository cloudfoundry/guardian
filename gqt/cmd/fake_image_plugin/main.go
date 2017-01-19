package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"

	"github.com/kardianos/osext"
)

func main() {
	fakeImagePlugin := cli.NewApp()
	fakeImagePlugin.Name = "fakeImagePlugin"
	fakeImagePlugin.Usage = "I am FakeImagePlugin!"

	fakeImagePlugin.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "image-path",
			Usage: "Path to use as image",
		},
		cli.StringFlag{
			Name:  "args-path",
			Usage: "Path to write args to",
		},
		cli.StringFlag{
			Name:  "create-whoami-path",
			Usage: "Path to write uid/gid to on create",
		},
		cli.StringFlag{
			Name:  "destroy-whoami-path",
			Usage: "Path to write uid/gid on destroy",
		},
		cli.StringFlag{
			Name:  "metrics-whoami-path",
			Usage: "Path to write uid/gid on metrics",
		},
		cli.StringFlag{
			Name:  "json-file-to-copy",
			Usage: "Path to json file to opy as image.json",
		},
		cli.StringFlag{
			Name:  "create-log-content",
			Usage: "Fake log content to write to stderr on create",
		},
		cli.StringFlag{
			Name:  "destroy-log-content",
			Usage: "Fake log content to write to stderr on destroy",
		},
		cli.StringFlag{
			Name:  "metrics-log-content",
			Usage: "Fake log content to write to stderr on metrics",
		},
		cli.StringFlag{
			Name:  "fail-on",
			Usage: "action to fail on",
		},
		cli.StringFlag{
			Name:  "metrics-output",
			Usage: "metrics json to print on stdout on metrics",
		},
		cli.StringFlag{
			Name:  "create-bin-location-path",
			Usage: "Path to write this binary's location to on create",
		},
		cli.StringFlag{
			Name:  "destroy-bin-location-path",
			Usage: "Path to write this binary's location to on destroy",
		},
		cli.StringFlag{
			Name:  "metrics-bin-location-path",
			Usage: "Path to write this binary's location to on metrics",
		},
	}

	fakeImagePlugin.Commands = []cli.Command{
		CreateCommand,
		DeleteCommand,
		StatsCommand,
	}

	_ = fakeImagePlugin.Run(os.Args)
}

var CreateCommand = cli.Command{
	Name: "create",
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "uid-mapping",
			Usage: "uid mappings",
		},
		cli.StringSliceFlag{
			Name:  "gid-mapping",
			Usage: "gid mappings",
		},
		cli.Int64Flag{
			Name:  "disk-limit-size-bytes",
			Usage: "disk limit quota",
		},
		cli.BoolFlag{
			Name:  "exclude-image-from-quota",
			Usage: "exclude base image from disk quota",
		},
	},

	Action: func(ctx *cli.Context) error {
		failOn := ctx.GlobalString("fail-on")
		if failOn == "create" {
			fmt.Println("create failed")
			os.Exit(10)
		}

		argsFile := ctx.GlobalString("args-path")
		if argsFile != "" {
			err := ioutil.WriteFile(argsFile, []byte(strings.Join(os.Args, " ")), 0777)
			if err != nil {
				panic(err)
			}
		}

		whoamiFile := ctx.GlobalString("create-whoami-path")
		if whoamiFile != "" {
			err := ioutil.WriteFile(whoamiFile, []byte(fmt.Sprintf("%d - %d", os.Getuid(), os.Getgid())), 0777)
			if err != nil {
				panic(err)
			}
		}

		binLocationFile := ctx.GlobalString("create-bin-location-path")
		if binLocationFile != "" {
			executable, err := osext.Executable()
			if err != nil {
				panic(err)
			}

			err = ioutil.WriteFile(binLocationFile, []byte(executable), 0777)
			if err != nil {
				panic(err)
			}
		}

		imagePath := ctx.GlobalString("image-path")
		if imagePath != "" {
			rootFSPath := filepath.Join(imagePath, "rootfs")
			if err := os.MkdirAll(rootFSPath, 0777); err != nil {
				panic(err)
			}
		}

		jsonFile := ctx.GlobalString("json-file-to-copy")
		if jsonFile != "" {
			if err := copyFile(jsonFile, filepath.Join(imagePath, "image.json")); err != nil {
				panic(err)
			}
		}

		logContent := ctx.GlobalString("create-log-content")
		if logContent != "" {
			log := lager.NewLogger("fake-image-plugin")
			log.RegisterSink(lager.NewWriterSink(os.Stderr, lager.INFO))
			log.Info(logContent)
		}

		fmt.Println(imagePath)

		return nil
	},
}

var DeleteCommand = cli.Command{
	Name: "delete",

	Action: func(ctx *cli.Context) error {
		failOn := ctx.GlobalString("fail-on")
		if failOn == "destroy" {
			fmt.Println("destroy failed")
			os.Exit(10)
		}

		argsFile := ctx.GlobalString("args-path")
		if argsFile != "" {
			err := ioutil.WriteFile(argsFile, []byte(strings.Join(os.Args, " ")), 0777)
			if err != nil {
				panic(err)
			}
		}

		whoamiFile := ctx.GlobalString("destroy-whoami-path")
		if whoamiFile != "" {
			err := ioutil.WriteFile(whoamiFile, []byte(fmt.Sprintf("%d - %d", os.Getuid(), os.Getgid())), 0777)
			if err != nil {
				panic(err)
			}
		}

		binLocationFile := ctx.GlobalString("destroy-bin-location-path")
		if binLocationFile != "" {
			executable, err := osext.Executable()
			if err != nil {
				panic(err)
			}

			f, err := os.Create(binLocationFile)
			if err != nil {
				panic(err)
			}
			f.Close()

			f, err = os.OpenFile(binLocationFile, os.O_APPEND|os.O_WRONLY, 0777)
			if err != nil {
				panic(err)
			}

			defer f.Close()

			if _, err = f.WriteString(executable); err != nil {
				panic(err)
			}
		}

		logContent := ctx.GlobalString("destroy-log-content")
		if logContent != "" {
			log := lager.NewLogger("fake-image-plugin")
			log.RegisterSink(lager.NewWriterSink(os.Stderr, lager.INFO))
			log.Info(logContent)
		}

		return nil
	},
}

var StatsCommand = cli.Command{
	Name: "stats",

	Action: func(ctx *cli.Context) error {
		failOn := ctx.GlobalString("fail-on")
		if failOn == "metrics" {
			fmt.Println("metrics failed")
			os.Exit(10)
		}
		argsFile := ctx.GlobalString("args-path")
		if argsFile != "" {
			err := ioutil.WriteFile(argsFile, []byte(strings.Join(os.Args, " ")), 0777)
			if err != nil {
				panic(err)
			}
		}

		whoamiFile := ctx.GlobalString("metrics-whoami-path")
		if whoamiFile != "" {
			err := ioutil.WriteFile(whoamiFile, []byte(fmt.Sprintf("%d - %d", os.Getuid(), os.Getgid())), 0777)
			if err != nil {
				panic(err)
			}
		}

		binLocationFile := ctx.GlobalString("metrics-bin-location-path")
		if binLocationFile != "" {
			executable, err := osext.Executable()
			if err != nil {
				panic(err)
			}

			err = ioutil.WriteFile(binLocationFile, []byte(executable), 0777)
			if err != nil {
				panic(err)
			}
		}

		logContent := ctx.GlobalString("metrics-log-content")
		if logContent != "" {
			log := lager.NewLogger("fake-image-plugin")
			log.RegisterSink(lager.NewWriterSink(os.Stderr, lager.INFO))
			log.Info(logContent)
		}

		metricsOutput := ctx.GlobalString("metrics-output")
		if metricsOutput != "" {
			fmt.Println(metricsOutput)
		} else {
			fmt.Println("{}")
		}

		return nil
	},
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
