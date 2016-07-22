package main

import (
	"os"

	"code.cloudfoundry.org/grootfs/commands"
	"code.cloudfoundry.org/grootfs/graph"
	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"
)

const GraphPath = "/tmp/grootfs"

func main() {
	grootfs := cli.NewApp()
	grootfs.Name = "grootfs"
	grootfs.Version = "0.0.0"

	grootfs.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "graph",
			Usage: "Path to the graph directory",
			Value: GraphPath,
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Set for verbose logging",
		},
	}

	grootfs.Commands = []cli.Command{
		commands.CreateCommand,
	}

	grootfs.Before = func(ctx *cli.Context) error {
		graphPath := ctx.String("graph")

		logger := lager.NewLogger("grootfs")
		logLevel := lager.INFO
		if ctx.Bool("debug") {
			logLevel = lager.DEBUG
		}
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, logLevel))
		ctx.App.Metadata["logger"] = logger

		configurer := graph.NewConfigurer()
		return configurer.Ensure(logger, graphPath)
	}

	grootfs.Run(os.Args)
}
