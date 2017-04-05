package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"encoding/json"
	"fmt"
	"os"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/commands/idfinder"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	imageClonerpkg "code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli"
)

var StatsCommand = cli.Command{
	Name:        "stats",
	Usage:       "stats [options] <id|image path>",
	Description: "Return filesystem stats",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("stats")
		newExitError := newErrorHandler(logger, "stats")

		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errorspkg.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return newExitError(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		cfg, err := configBuilder.Build()
		logger.Debug("stats-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return newExitError(err.Error(), 1)
		}

		storePath := cfg.StorePath
		idOrPath := ctx.Args().First()
		id, err := idfinder.FindID(storePath, idOrPath)
		if err != nil {
			logger.Error("find-id-failed", err, lager.Data{"id": idOrPath, "storePath": storePath})
			return newExitError(err.Error(), 1)
		}

		fsDriver, err := createFileSystemDriver(cfg)
		if err != nil {
			return newExitError(err.Error(), 1)
		}
		imageCloner := imageClonerpkg.NewImageCloner(fsDriver, storePath)

		metricsEmitter := metrics.NewEmitter()
		statser := groot.IamStatser(imageCloner, metricsEmitter)
		stats, err := statser.Stats(logger, id)
		if err != nil {
			logger.Error("fetching-stats", err)
			return newExitError(err.Error(), 1)
		}

		json.NewEncoder(os.Stdout).Encode(stats)
		return nil
	},
}
