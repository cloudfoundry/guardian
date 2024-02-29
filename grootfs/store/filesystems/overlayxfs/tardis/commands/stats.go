package commands // import "code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis/commands"

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis/stats"
	"code.cloudfoundry.org/lager/v3"
	"github.com/urfave/cli/v2"
)

var StatsCommand = cli.Command{
	Name:        "stats",
	Usage:       "stats --volume-path <path>",
	Description: "Get stats for a volume",

	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "volume-path",
			Usage: "Path to the volume",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := lager.NewLogger("tardis")
		logger.RegisterSink(createLoggingSink(os.Stderr, lager.DEBUG, ctx.String("log-timestamp-format")))

		volumeStats, err := stats.VolumeStats(
			logger,
			ctx.String("volume-path"),
		)

		if err != nil {
			logger.Error("fetching-volume-stats", err)
			return cli.Exit(err.Error(), 1)
		}

		jsonStats, err := json.Marshal(volumeStats)
		if err != nil {
			logger.Error("marshaling-volume-stats", err)
			return cli.Exit(err.Error(), 2)
		}

		if _, err := os.Stdout.Write(jsonStats); err != nil {
			logger.Error("printing-stats", err)
			return cli.Exit(err.Error(), 3)
		}

		return nil
	},
}
