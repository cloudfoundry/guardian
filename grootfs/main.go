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

const defaultStorePath = "/var/lib/grootfs"
const defaultDraxBin = "drax"

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
		logFile := ctx.GlobalString("log-file")
		logLevel := ctx.String("log-level")

		lagerLogLevel := translateLogLevel(logLevel)
		logger, err := configureLogger(lagerLogLevel, logFile)
		if err != nil {
			return err
		}
		ctx.App.Metadata["logger"] = logger

		cfgBuilder, err := newConfigBuilder(ctx.GlobalString("config"))
		if err != nil {
			logger.Error("failed-loading-config-file", err)
			return cli.NewExitError(err.Error(), 1)
		}
		cfgBuilder.WithStorePath(ctx.GlobalString("store"), defaultStorePath).
			WithDraxBin(ctx.GlobalString("drax-bin"), defaultDraxBin).
			WithMetronEndpoint(ctx.GlobalString("metron-endpoint"))
		ctx.App.Metadata["configBuilder"] = cfgBuilder

		// Sadness. We need to do that becuase we use stderr for logs so user
		// errors need to end up in stdout.
		cli.ErrWriter = os.Stdout

		configurer := store.NewConfigurer()
		cfg := cfgBuilder.Build()
		if err := configurer.Ensure(logger, cfg.UserBasedStorePath); err != nil {
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

func loadConfig(configPath string) (config.Config, error) {
	var cfg config.Config

	if configPath != "" {
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

func newConfigBuilder(configPath string) (*config.Builder, error) {
	if configPath != "" {
		cfgBuilder, err := config.NewBuilderFromFile(configPath)
		if err != nil {
			return cfgBuilder, err
		}

		return cfgBuilder, nil
	}

	return config.NewBuilder(), nil
}
