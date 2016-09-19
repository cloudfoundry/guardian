package commands

import (
	"os"

	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	"code.cloudfoundry.org/grootfs/store/volume_driver/drax/metrix"
	"code.cloudfoundry.org/lager"
	"github.com/urfave/cli"
)

var MetricsCommand = cli.Command{
	Name:        "metrics",
	Usage:       "metrics --volume-path <path> [--force-sync]",
	Description: "Get metrics for a volume",

	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "volume-path",
			Usage: "Path to the volume",
		},
		cli.BoolFlag{
			Name:  "force-sync",
			Usage: "Force BTRFS to update metrics immediately",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := lager.NewLogger("drax")
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

		commandRunner := linux_command_runner.New()
		metrix := metrix.NewBtrfsMetrics(commandRunner)
		metrics, err := metrix.VolumeMetrics(
			logger,
			ctx.String("volume-path"),
			ctx.Bool("force-sync"),
		)

		if err != nil {
			logger.Error("fetching-volume-metrics", err)
			return cli.NewExitError(err.Error(), 1)
		}

		if _, err := os.Stdout.Write(metrics); err != nil {
			logger.Error("encoding-metrics", err)
			return cli.NewExitError(err.Error(), 1)
		}

		return nil
	},
}
