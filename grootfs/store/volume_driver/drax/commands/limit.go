package commands

import (
	"os"

	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	limiterpkg "code.cloudfoundry.org/grootfs/store/volume_driver/drax/limiter"
	"code.cloudfoundry.org/lager"
	"github.com/urfave/cli"
)

var LimitCommand = cli.Command{
	Name:        "limit",
	Usage:       "limit --disk-limit-bytes 102400 --volume-path <path>",
	Description: "Add disk limits to the volume.",

	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "volume-path",
			Usage: "Path to the volume",
		},
		cli.Int64Flag{
			Name:  "disk-limit-bytes",
			Usage: "Disk limit in bytes",
		},
		cli.BoolFlag{
			Name:  "exclude-image-from-quota",
			Usage: "Exclude base image from disk quota",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := lager.NewLogger("drax")
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

		commandRunner := linux_command_runner.New()
		limiter := limiterpkg.NewBtrfsLimiter(commandRunner)
		err := limiter.ApplyDiskLimit(
			logger,
			ctx.String("volume-path"),
			ctx.Int64("disk-limit-bytes"),
			ctx.Bool("exclude-image-from-quota"),
		)
		if err != nil {
			logger.Error("applying-limit", err)
			return cli.NewExitError(err.Error(), 1)
		}

		return nil
	},
}
