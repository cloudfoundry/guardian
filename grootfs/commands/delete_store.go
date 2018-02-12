package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/store/locksmith"
	"code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/lager"
	"github.com/urfave/cli"
)

var DeleteStoreCommand = cli.Command{
	Name:        "delete-store",
	Usage:       "delete-store --store <path>",
	Description: "Deletes the given store from the system",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("delete-store")

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		cfg, err := configBuilder.Build()
		logger.Debug("delete-store", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fsDriver, err := createFileSystemDriver(cfg)
		if err != nil {
			logger.Error("failed-to-initialise-filesystem-driver", err)
			return cli.NewExitError(err.Error(), 1)
		}

		storePath := cfg.StorePath
		locksmith := locksmith.NewSharedFileSystem(storePath, metrics.NewEmitter(logger, cfg.MetronEndpoint))
		manager := manager.New(storePath, nil, fsDriver, fsDriver, fsDriver)

		if err := manager.DeleteStore(logger, locksmith); err != nil {
			logger.Error("cleaning-up-store-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		return nil
	},
}
