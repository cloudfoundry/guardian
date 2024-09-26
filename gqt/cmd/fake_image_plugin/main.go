package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager/v3"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli/v2"

	"github.com/kardianos/osext"
)

func main() {
	fakeImagePlugin := cli.NewApp()
	fakeImagePlugin.Name = "fakeImagePlugin"
	fakeImagePlugin.Usage = "I am FakeImagePlugin!"

	fakeImagePlugin.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "rootfs-path",
			Usage: "Path to use as rootfs",
		},
		&cli.StringFlag{
			Name:  "args-path",
			Usage: "Path to write args to",
		},
		&cli.StringFlag{
			Name:  "create-whoami-path",
			Usage: "Path to write uid/gid to on create",
		},
		&cli.StringFlag{
			Name:  "destroy-whoami-path",
			Usage: "Path to write uid/gid on destroy",
		},
		&cli.StringFlag{
			Name:  "metrics-whoami-path",
			Usage: "Path to write uid/gid on metrics",
		},
		&cli.StringFlag{
			Name:  "image-json",
			Usage: "Image json to use as image json",
		},
		&cli.StringFlag{
			Name:  "env-json",
			Usage: "environment container processes should inherit",
		},
		&cli.StringFlag{
			Name:  "mounts-json",
			Usage: "Mounts json",
		},
		&cli.StringFlag{
			Name:  "create-log-content",
			Usage: "Fake log content to write to stderr on create",
		},
		&cli.StringFlag{
			Name:  "destroy-log-content",
			Usage: "Fake log content to write to stderr on destroy",
		},
		&cli.StringFlag{
			Name:  "metrics-log-content",
			Usage: "Fake log content to write to stderr on metrics",
		},
		&cli.StringFlag{
			Name:  "fail-on",
			Usage: "action to fail on",
		},
		&cli.StringFlag{
			Name:  "metrics-output",
			Usage: "metrics json to print on stdout on metrics",
		},
		&cli.StringFlag{
			Name:  "create-bin-location-path",
			Usage: "Path to write this binary's location to on create",
		},
		&cli.StringFlag{
			Name:  "destroy-bin-location-path",
			Usage: "Path to write this binary's location to on destroy",
		},
		&cli.StringFlag{
			Name:  "metrics-bin-location-path",
			Usage: "Path to write this binary's location to on metrics",
		},
		&cli.BoolFlag{
			Name:  "old-return-schema",
			Usage: "use old, deprecated DesiredImageSpec as return json",
		},
		&cli.StringFlag{
			Name:  "store",
			Usage: "path to store. Not used by fake, but needed for delete-store not to fail.",
		},
	}

	fakeImagePlugin.Commands = []*cli.Command{
		&CreateCommand,
		&DeleteCommand,
		&StatsCommand,
		&DeleteStoreCommand,
	}

	_ = fakeImagePlugin.Run(os.Args)
}

var CreateCommand = cli.Command{
	Name: "create",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "uid-mapping",
			Usage: "uid mappings",
		},
		&cli.StringSliceFlag{
			Name:  "gid-mapping",
			Usage: "gid mappings",
		},
		&cli.Int64Flag{
			Name:  "disk-limit-size-bytes",
			Usage: "disk limit quota",
		},
		&cli.BoolFlag{
			Name:  "exclude-image-from-quota",
			Usage: "exclude base image from disk quota",
		},
		&cli.StringFlag{
			Name:  "username",
			Usage: "username for docker private registry",
		},
		&cli.StringFlag{
			Name:  "password",
			Usage: "password for docker private registry",
		},
	},

	Action: func(ctx *cli.Context) error {
		failOn := ctx.String("fail-on")
		if failOn == "create" {
			fmt.Println("create failed")
			os.Exit(10)
		}

		argsFile := ctx.String("args-path")
		if argsFile != "" {
			err := os.WriteFile(argsFile, []byte(strings.Join(os.Args, " ")), 0644)
			if err != nil {
				panic(err)
			}
		}

		whoamiFile := ctx.String("create-whoami-path")
		if whoamiFile != "" {
			err := os.WriteFile(whoamiFile, []byte(fmt.Sprintf("%d - %d", os.Getuid(), os.Getgid())), 0644)
			if err != nil {
				panic(err)
			}
		}

		binLocationFile := ctx.String("create-bin-location-path")
		if binLocationFile != "" {
			executable, err := osext.Executable()
			if err != nil {
				panic(err)
			}

			err = os.WriteFile(binLocationFile, []byte(executable), 0644)
			if err != nil {
				panic(err)
			}
		}

		rootfsPath := ctx.String("rootfs-path")
		if rootfsPath != "" {
			rootFSPath := filepath.Join(rootfsPath, "rootfs")
			if err := os.MkdirAll(rootFSPath, 0755); err != nil {
				panic(err)
			}
		}

		var mounts []specs.Mount
		mountsJson := ctx.String("mounts-json")
		if mountsJson != "" {
			if err := json.Unmarshal([]byte(mountsJson), &mounts); err != nil {
				panic(err)
			}
		}

		var env []string
		envJson := ctx.String("env-json")
		if envJson != "" {
			if err := json.Unmarshal([]byte(envJson), &env); err != nil {
				panic(err)
			}
		}

		logContent := ctx.String("create-log-content")
		if logContent != "" {
			log := lager.NewLogger("fake-image-plugin")
			log.RegisterSink(lager.NewWriterSink(os.Stderr, lager.INFO))
			log.Info(logContent)
		}

		output := specs.Spec{
			Root: &specs.Root{
				Path: rootfsPath,
			},
			Mounts: mounts,
			Process: &specs.Process{
				Env: env,
			},
			Windows: &specs.Windows{
				LayerFolders: []string{"layer", "folders"},
			},
		}

		b, err := json.Marshal(output)
		if err != nil {
			panic(err)
		}

		fmt.Println(string(b))

		return nil
	},
}

var DeleteCommand = cli.Command{
	Name: "delete",

	Action: func(ctx *cli.Context) error {
		failOn := ctx.String("fail-on")
		if failOn == "destroy" {
			fmt.Println("destroy failed")
			os.Exit(10)
		}

		argsFile := ctx.String("args-path")
		if argsFile != "" {
			err := os.WriteFile(argsFile, []byte(strings.Join(os.Args, " ")), 0644)
			if err != nil {
				panic(err)
			}
		}

		whoamiFile := ctx.String("destroy-whoami-path")
		if whoamiFile != "" {
			err := os.WriteFile(whoamiFile, []byte(fmt.Sprintf("%d - %d", os.Getuid(), os.Getgid())), 0644)
			if err != nil {
				panic(err)
			}
		}

		binLocationFile := ctx.String("destroy-bin-location-path")
		if binLocationFile != "" {
			executable, err := osext.Executable()
			if err != nil {
				panic(err)
			}

			f, err := os.Create(binLocationFile)
			if err != nil {
				panic(err)
			}
			err = f.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to close file %s: %s\n", f.Name(), err)
			}

			f, err = os.OpenFile(binLocationFile, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				panic(err)
			}

			defer f.Close()

			if _, err = f.WriteString(executable); err != nil {
				panic(err)
			}
		}

		logContent := ctx.String("destroy-log-content")
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
		failOn := ctx.String("fail-on")
		if failOn == "metrics" {
			fmt.Println("metrics failed")
			os.Exit(10)
		}
		argsFile := ctx.String("args-path")
		if argsFile != "" {
			err := os.WriteFile(argsFile, []byte(strings.Join(os.Args, " ")), 0644)
			if err != nil {
				panic(err)
			}
		}

		whoamiFile := ctx.String("metrics-whoami-path")
		if whoamiFile != "" {
			err := os.WriteFile(whoamiFile, []byte(fmt.Sprintf("%d - %d", os.Getuid(), os.Getgid())), 0644)
			if err != nil {
				panic(err)
			}
		}

		binLocationFile := ctx.String("metrics-bin-location-path")
		if binLocationFile != "" {
			executable, err := osext.Executable()
			if err != nil {
				panic(err)
			}

			err = os.WriteFile(binLocationFile, []byte(executable), 0644)
			if err != nil {
				panic(err)
			}
		}

		logContent := ctx.String("metrics-log-content")
		if logContent != "" {
			log := lager.NewLogger("fake-image-plugin")
			log.RegisterSink(lager.NewWriterSink(os.Stderr, lager.INFO))
			log.Info(logContent)
		}

		metricsOutput := ctx.String("metrics-output")
		if metricsOutput != "" {
			fmt.Println(metricsOutput)
		} else {
			fmt.Println("{}")
		}

		return nil
	},
}

var DeleteStoreCommand = cli.Command{
	Name: "delete-store",

	Action: func(ctx *cli.Context) error {
		return nil
	},
}
