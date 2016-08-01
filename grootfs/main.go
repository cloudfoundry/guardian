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
	grootfs.Usage = "I am Groot!"
	grootfs.Version = "0.0.0"

	grootfs.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "graph",
			Usage: "Path to the graph directory",
			Value: GraphPath,
		},
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "Toggles logging",
		},
	}

	grootfs.Commands = []cli.Command{
		commands.CreateCommand,
		commands.DeleteCommand,
		commands.UntarCommand,
	}

	grootfs.Before = func(ctx *cli.Context) error {
		graphPath := ctx.String("graph")
		cli.ErrWriter = os.Stdout

		logger := lager.NewLogger("grootfs")
		logLevel := lager.FATAL
		if ctx.Bool("verbose") {
			logLevel = lager.DEBUG
		}
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, logLevel))
		ctx.App.Metadata["logger"] = logger

		configurer := graph.NewConfigurer()
		return configurer.Ensure(logger, graphPath)
	}

	grootfs.Run(os.Args)
}
