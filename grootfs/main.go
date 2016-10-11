package main

import (
	"os"
	"strings"

	"code.cloudfoundry.org/grootfs/commands"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"

	"github.com/docker/docker/pkg/reexec"
	"github.com/urfave/cli"
)

const DefaultStorePath = "/var/lib/grootfs"

func init() {
	if reexec.Init() {
		os.Exit(0)
	}
}

func main() {
	grootfs := cli.NewApp()
	grootfs.Name = "grootfs"
	grootfs.Usage = "I am Groot!"
	grootfs.Version = "0.1.0"

	grootfs.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "store",
			Usage: "Path to the store directory",
			Value: DefaultStorePath,
		},
		cli.StringFlag{
			Name:  "log-level",
			Usage: "Set logging level. <info|debug|error|fatal>",
			Value: "fatal",
		},
		cli.StringFlag{
			Name:  "log-file",
			Usage: "Forward logs to file",
		},
		cli.StringFlag{
			Name:  "drax-bin",
			Usage: "Path to drax bin. (If not provided will use $PATH)",
			Value: "drax",
		},
	}

	grootfs.Commands = []cli.Command{
		commands.CreateCommand,
		commands.DeleteCommand,
		commands.MetricsCommand,
	}

	grootfs.Before = func(ctx *cli.Context) error {
		storePath := ctx.String("store")

		cli.ErrWriter = os.Stdout

		logger := configureLog(ctx)

		configurer := store.NewConfigurer()
		return configurer.Ensure(logger, storePath)
	}

	grootfs.Run(os.Args)
}

func configureLog(ctx *cli.Context) lager.Logger {
	logFile := ctx.GlobalString("log-file")
	logLevel := ctx.String("log-level")
	logWriter := os.Stderr

	if logFile != "" {
		logWriter, _ = os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	}

	logger := lager.NewLogger("grootfs")
	lagerLogLevel := translateLogLevel(logLevel)

	logger.RegisterSink(lager.NewWriterSink(logWriter, lagerLogLevel))
	ctx.App.Metadata["logger"] = logger

	return logger
}

func translateLogLevel(logLevel string) lager.LogLevel {
	switch strings.ToUpper(logLevel) {
	case "DEBUG":
		return lager.DEBUG
	case "INFO":
		return lager.INFO
	case "ERROR":
		return lager.ERROR
	default:
		return lager.FATAL
	}
}
