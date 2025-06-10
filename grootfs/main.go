package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/grootfs/commands"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager/v3"

	"github.com/containers/storage/pkg/reexec"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

const (
	defaultTardisBin    = "tardis"
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
	grootfs.Version = version
	grootfs.Usage = "I am Groot!"

	/* This block is copied from here:
		https://github.com/cloudfoundry/grootfs/blob/e3748cdda45b5e7efe500e442f9ae029499a287d/vendor/github.com/urfave/cli/v2/errors.go#L94-L116

		Here is why:
	    The default ExitErrHandler is always printing error messagees to stderr. However, Grootfs is using stderr
		for logging unless a log file is supplied, and error messages are expected by the test runner on stdout. Given that
		the default handler does not allow configuration of the error stream we have created one that is identical, but writes
		error messges to stdout to keep our tests happy.
	*/
	grootfs.ExitErrHandler = func(ctx *cli.Context, err error) {
		if err == nil {
			return
		}

		if exitErr, ok := err.(cli.ExitCoder); ok {
			if err.Error() != "" {
				if _, ok := exitErr.(cli.ErrorFormatter); ok {
					_, _ = fmt.Fprintf(grootfs.Writer, "%+v\n", err)
				} else {
					_, _ = fmt.Fprintln(grootfs.Writer, err)
				}
			}
			cli.OsExiter(exitErr.ExitCode())
			return
		}
	}

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
			return cli.Exit(err.Error(), 1)
		}
		ctx.App.Metadata["configBuilder"] = cfgBuilder

		cfg, err := cfgBuilder.WithStorePath(ctx.String("store"), ctx.IsSet("store")).
			WithTardisBin(ctx.String("tardis-bin"), ctx.IsSet("tardis-bin")).
			WithMetronEndpoint(ctx.String("metron-endpoint")).
			WithLogLevel(ctx.String("log-level"), ctx.IsSet("log-level")).
			WithLogFile(ctx.String("log-file")).
			WithNewuidmapBin(ctx.String("newuidmap-bin"), ctx.IsSet("newuidmap-bin")).
			WithNewgidmapBin(ctx.String("newgidmap-bin"), ctx.IsSet("newgidmap-bin")).
			Build()
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}

		lagerLogLevel := translateLogLevel(cfg.LogLevel)
		logger, err := configureLogger(lagerLogLevel, cfg.LogFile)
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}
		ctx.App.Metadata["logger"] = logger

		// Sadness. We need to do that becuase we use stderr for logs so user
		// errors need to end up in stdout.
		cli.ErrWriter = os.Stdout

		if err := os.Setenv("TMPDIR", filepath.Join(cfg.StorePath, store.TempDirName)); err != nil {
			logger.Error("setting TMPDIR env var", err)
			return cli.Exit(err.Error(), 1)
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

func configureLogger(logLevel lager.LogLevel, logFile string) (lager.Logger, error) {
	logWriter := os.Stderr
	if logFile != "" {
		var err error
		logWriter, err = os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			return nil, errors.Wrap(err, "failed to configure logger")
		}

		if logLevel == lager.FATAL {
			logLevel = lager.INFO
		}
	}

	logger := lager.NewLogger("grootfs")
	logger.RegisterSink(lager.NewPrettySink(logWriter, logLevel))

	return logger, nil
}
