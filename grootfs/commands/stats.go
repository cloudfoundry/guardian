package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/commands/idfinder"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	imageClonerpkg "code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/grootfs/store/volume_driver"
	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"
)

var StatsCommand = cli.Command{
	Name:        "stats",
	Usage:       "stats [options] <id>",
	Description: "Return filesystem stats",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("stats")

		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errors.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return cli.NewExitError(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		cfg := configBuilder.Build()

		storePath := cfg.BaseStorePath
		idOrPath := ctx.Args().First()
		id, err := idfinder.FindID(storePath, idOrPath)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		if id == idOrPath {
			storePath = cfg.UserBasedStorePath
		} else {
			storePath, err = idfinder.FindSubStorePath(storePath, idOrPath)
			if err != nil {
				return cli.NewExitError(fmt.Sprintf("can't determine the store path: %s", err.Error()), 1)
			}
		}

		btrfsVolumeDriver := volume_driver.NewBtrfs(ctx.GlobalString("drax-bin"), storePath)
		imageCloner := imageClonerpkg.NewImageCloner(btrfsVolumeDriver, storePath)

		metricsEmitter := metrics.NewEmitter()
		statser := groot.IamStatser(imageCloner, metricsEmitter)
		stats, err := statser.Stats(logger, id)
		if err != nil {
			logger.Error("fetching-stats", err)
			return cli.NewExitError(err.Error(), 1)
		}

		json.NewEncoder(os.Stdout).Encode(stats)
		return nil
	},
}
