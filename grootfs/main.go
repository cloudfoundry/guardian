package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/commands"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"

	"github.com/containers/storage/pkg/reexec"
	"github.com/urfave/cli/v2"
)

const (
	defaultTardisBin    = "tardis"
	defaultNewuidmapBin = "newuidmap"
	defaultNewgidmapBin = "newgidmap"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	if reexec.Init() {
		os.Exit(0)
	}
}

func main() {
	grootfs := cli.NewApp()
	grootfs.Name = "grootfs"
	grootfs.Version = version
	grootfs.Usage = "I am Groot!"

	grootfs.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "config",
			Usage: "Path to config file",
		},
		&cli.StringFlag{
			Name:  "store",
			Usage: "Path to the store directory",
			Value: store.DefaultStorePath,
		},
		&cli.StringFlag{
			Name:  "log-level",
			Usage: "Set logging level <debug|info|error|fatal>",
			Value: "fatal",
		},
		&cli.StringFlag{
			Name:  "log-file",
			Usage: "File to write logs to. Using this option sets the log level to `info` if --log-level is not specified.",
		},
		&cli.StringFlag{
			Name:  "log-timestamp-format",
			Usage: "The format to use for timestamps in logs. Can be 'unix-epoch' or 'rfc3339'",
		},
		&cli.StringFlag{
			Name:  "tardis-bin",
			Usage: "Path to tardis bin. (If not provided will use $PATH)",
			Value: defaultTardisBin,
		},
		&cli.StringFlag{
			Name:  "newuidmap-bin",
			Usage: "Path to newuidmap bin. (If not provided will use $PATH)",
			Value: defaultNewuidmapBin,
		},
		&cli.StringFlag{
			Name:  "newgidmap-bin",
			Usage: "Path to newgidmap bin. (If not provided will use $PATH)",
			Value: defaultNewgidmapBin,
		},
		&cli.StringFlag{
			Name:  "metron-endpoint",
			Usage: "Metron endpoint used to send metrics",
			Value: "",
		},
	}

	grootfs.Commands = []*cli.Command{
		&commands.InitStoreCommand,
		&commands.DeleteStoreCommand,
		&commands.GenerateVolumeSizeMetadata,
		&commands.CreateCommand,
		&commands.DeleteCommand,
		&commands.StatsCommand,
		&commands.CleanCommand,
		&commands.ListCommand,
		&commands.CapacityCommand,
	}

	grootfs.Before = func(ctx *cli.Context) error {
		cfgBuilder, err := config.NewBuilder(ctx.String("config"))
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		ctx.App.Metadata["configBuilder"] = cfgBuilder

		cfg, err := cfgBuilder.WithStorePath(ctx.String("store"), ctx.IsSet("store")).
			WithTardisBin(ctx.String("tardis-bin"), ctx.IsSet("tardis-bin")).
			WithMetronEndpoint(ctx.String("metron-endpoint")).
			WithLogLevel(ctx.String("log-level"), ctx.IsSet("log-level")).
			WithLogFile(ctx.String("log-file")).
			WithLogTimestampFormat(ctx.String("log-timestamp-format")).
			WithNewuidmapBin(ctx.String("newuidmap-bin"), ctx.IsSet("newuidmap-bin")).
			WithNewgidmapBin(ctx.String("newgidmap-bin"), ctx.IsSet("newgidmap-bin")).
			Build()
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		lagerLogLevel := translateLogLevel(cfg.LogLevel)
		logger, err := configureLogger(lagerLogLevel, cfg.LogFile, cfg.LogTimestampFormat)
		if err != nil {
			return err
		}
		ctx.App.Metadata["logger"] = logger

		// Sadness. We need to do that becuase we use stderr for logs so user
		// errors need to end up in stdout.
		cli.ErrWriter = os.Stdout

		if err := os.Setenv("TMPDIR", filepath.Join(cfg.StorePath, store.TempDirName)); err != nil {
			logger.Error("setting TMPDIR env var", err)
			return cli.NewExitError(err.Error(), 1)
		}

		return nil
	}

	if err := grootfs.Run(os.Args); err != nil {
		os.Exit(1)
	}
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

func configureLogger(logLevel lager.LogLevel, logFile, logTimestampFormat string) (lager.Logger, error) {
	logWriter := os.Stderr
	if logFile != "" {
		var err error
		logWriter, err = os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			return nil, err
		}

		if logLevel == lager.FATAL {
			logLevel = lager.INFO
		}
	}

	logger := lager.NewLogger("grootfs")
	if logTimestampFormat == "rfc3339" {
		logger.RegisterSink(lager.NewPrettySink(logWriter, logLevel))
	} else {
		logger.RegisterSink(lager.NewWriterSink(logWriter, logLevel))
	}

	return logger, nil
}
