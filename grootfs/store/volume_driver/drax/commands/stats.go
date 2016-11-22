package commands // import "code.cloudfoundry.org/grootfs/store/volume_driver/drax/commands"

import (
	"os"

	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	"code.cloudfoundry.org/grootfs/store/volume_driver/drax/metrix"
	"code.cloudfoundry.org/lager"
	"github.com/urfave/cli"
)

var StatsCommand = cli.Command{
	Name:        "stats",
	Usage:       "stats --volume-path <path> [--force-sync]",
	Description: "Get stats for a volume",

	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "volume-path",
			Usage: "Path to the volume",
		},
		cli.BoolFlag{
			Name:  "force-sync",
			Usage: "Force BTRFS to update stats immediately",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := lager.NewLogger("drax")
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

		commandRunner := linux_command_runner.New()
		metrix := metrix.NewBtrfsStats(commandRunner)
		stats, err := metrix.VolumeStats(
			logger,
			ctx.String("volume-path"),
			ctx.Bool("force-sync"),
		)

		if err != nil {
			logger.Error("fetching-volume-stats", err)
			return cli.NewExitError(err.Error(), 1)
		}

		if _, err := os.Stdout.Write(stats); err != nil {
			logger.Error("encoding-stats", err)
			return cli.NewExitError(err.Error(), 1)
		}

		return nil
	},
}
