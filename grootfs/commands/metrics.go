package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	grootpkg "code.cloudfoundry.org/grootfs/groot"
	storepkg "code.cloudfoundry.org/grootfs/store"
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
		logger = logger.Session("create")

		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errors.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return cli.NewExitError(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		storePath := ctx.GlobalString("store")
		id := ctx.Args().First()

		btrfsVolumeDriver := volume_driver.NewBtrfs(storePath)
		bundler := storepkg.NewBundler(btrfsVolumeDriver, storePath)

		groot := grootpkg.IamGroot(bundler, nil, nil)
		metrics, err := groot.Metrics(logger, id)
		if err != nil {
			logger.Error("fetching-metrics", err)
			return cli.NewExitError(err.Error(), 1)
		}

		json.NewEncoder(os.Stdout).Encode(metrics)
		return nil
	},
}
