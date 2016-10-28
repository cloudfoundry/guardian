package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"code.cloudfoundry.org/grootfs/commands/storepath"
	"code.cloudfoundry.org/grootfs/groot"
	bundlerpkg "code.cloudfoundry.org/grootfs/store/bundler"
	"code.cloudfoundry.org/grootfs/store/volume_driver"
	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"
)

var MetricsCommand = cli.Command{
	Name:        "metrics",
	Usage:       "metrics [options] <id>",
	Description: "Return filesystem metrics",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("metrics")

		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errors.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return cli.NewExitError(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		storePath, err := storepath.UserBased(ctx.GlobalString("store"))
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("can't determine the store path: %s", err.Error()), 1)
		}
		id := ctx.Args().First()

		btrfsVolumeDriver := volume_driver.NewBtrfs(ctx.GlobalString("drax-bin"), storePath)
		bundler := bundlerpkg.NewBundler(btrfsVolumeDriver, storePath)

		metricser := groot.IamMetricser(bundler)
		metrics, err := metricser.Metrics(logger, id)
		if err != nil {
			logger.Error("fetching-metrics", err)
			return cli.NewExitError(err.Error(), 1)
		}

		json.NewEncoder(os.Stdout).Encode(metrics)
		return nil
	},
}
