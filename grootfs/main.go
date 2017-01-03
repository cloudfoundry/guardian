package main

import (
	"os"
	"strings"

	"code.cloudfoundry.org/grootfs/commands"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/dropsonde"
	"github.com/docker/docker/pkg/reexec"
	"github.com/urfave/cli"
)

const (
	defaultBtrfsBin     = "btrfs"
	defaultDraxBin      = "drax"
	defaultStorePath    = "/var/lib/grootfs"
	defaultNewuidmapBin = "newuidmap"
	defaultNewgidmapBin = "newgidmap"
)

func init() {
	if reexec.Init() {
		os.Exit(0)
	}
}

func main() {
	grootfs := cli.NewApp()
	grootfs.Name = "grootfs"
	grootfs.Usage = "I am Groot!"

	grootfs.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config",
			Usage: "Path to config file",
		},
		cli.StringFlag{
			Name:  "store",
			Usage: "Path to the store directory",
			Value: defaultStorePath,
		},
		cli.StringFlag{
			Name:  "log-level",
			Usage: "Set logging level <debug|info|error|fatal>",
			Value: "fatal",
		},
		cli.StringFlag{
			Name:  "log-file",
			Usage: "Forward logs to file",
		},
		cli.StringFlag{
			Name:  "drax-bin",
			Usage: "Path to drax bin. (If not provided will use $PATH)",
			Value: defaultDraxBin,
		},
		cli.StringFlag{
			Name:  "btrfs-bin",
			Usage: "Path to btrfs bin. (If not provided will use $PATH)",
			Value: defaultBtrfsBin,
		},
		cli.StringFlag{
			Name:  "newuidmap-bin",
			Usage: "Path to newuidmap bin. (If not provided will use $PATH)",
			Value: defaultNewuidmapBin,
		},
		cli.StringFlag{
			Name:  "newgidmap-bin",
			Usage: "Path to newgidmap bin. (If not provided will use $PATH)",
			Value: defaultNewgidmapBin,
		},
		cli.StringFlag{
			Name:  "metron-endpoint",
			Usage: "Metron endpoint used to send metrics",
			Value: "",
		},
	}

	grootfs.Commands = []cli.Command{
		commands.CreateCommand,
		commands.DeleteCommand,
		commands.StatsCommand,
		commands.CleanCommand,
		commands.ListCommand,
	}

	grootfs.Before = func(ctx *cli.Context) error {
		cfgBuilder, err := config.NewBuilder(ctx.GlobalString("config"))
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		cfg, err := cfgBuilder.WithStorePath(ctx.GlobalString("store"), ctx.IsSet("store")).
			WithDraxBin(ctx.GlobalString("drax-bin"), ctx.IsSet("drax-bin")).
			WithMetronEndpoint(ctx.GlobalString("metron-endpoint")).
			WithLogLevel(ctx.GlobalString("log-level"), ctx.IsSet("log-level")).
			WithLogFile(ctx.GlobalString("log-file")).
			WithBtrfsBin(ctx.GlobalString("btrfs-bin"), ctx.IsSet("btrfs-bin")).
			WithNewuidmapBin(ctx.GlobalString("newuidmap-bin"), ctx.IsSet("newuidmap-bin")).
			WithNewgidmapBin(ctx.GlobalString("newgidmap-bin"), ctx.IsSet("newgidmap-bin")).
			Build()
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		ctx.App.Metadata["configBuilder"] = cfgBuilder

		lagerLogLevel := translateLogLevel(cfg.LogLevel)
		logger, err := configureLogger(lagerLogLevel, cfg.LogFile)
		if err != nil {
			return err
		}
		ctx.App.Metadata["logger"] = logger

		// Sadness. We need to do that becuase we use stderr for logs so user
		// errors need to end up in stdout.
		cli.ErrWriter = os.Stdout

		if err := configureStore(logger, cfg.UserBasedStorePath, ctx.Args()); err != nil {
			return err
		}

		dropsondeOrigin := grootfs.Name
		if cfg.MetronEndpoint != "" {
			if err := dropsonde.Initialize(cfg.MetronEndpoint, dropsondeOrigin); err != nil {
				logger.Error("failed-to-initialize-metrics-emitter", err)
			}
		}

		return nil
	}

	_ = grootfs.Run(os.Args)
}

func configureStore(logger lager.Logger, storePath string, args []string) error {
	var data lager.Data
	image := args[len(args)-1]
	if image != args[0] {
		data = lager.Data{"image": image}
	}

	configurer := store.NewConfigurer()
	if err := configurer.Ensure(logger, storePath); err != nil {
		logger.Error("failed-to-setup-store", err, data)
		return err
	}

	return nil
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

func configureLogger(logLevel lager.LogLevel, logFile string) (lager.Logger, error) {
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
	logger.RegisterSink(lager.NewWriterSink(logWriter, logLevel))

	return logger, nil
}
