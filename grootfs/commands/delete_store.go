package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"os"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/store/locksmith"
	"code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/lager"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var DeleteStoreCommand = cli.Command{
	Name:        "delete-store",
	Usage:       "delete-store",
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

		if _, err := os.Stat(cfg.StorePath); os.IsNotExist(err) {
			logger.Error("store path doesn't exist", err)
			return cli.NewExitError(errors.Wrap(err, "store path doesn't exist").Error(), 1)
		}

		fsDriver, err := createFileSystemDriver(cfg)
		if err != nil {
			logger.Error("failed-to-initialise-driver", err)
			return cli.NewExitError(err.Error(), 1)
		}

		storePath := cfg.StorePath
		locksmith := locksmith.NewFileSystem(storePath, metrics.NewEmitter())
		manager := manager.New(storePath, locksmith, fsDriver, fsDriver, fsDriver)
		var _ = manager.ConfigureStore(logger, 0, 0)

		if err := manager.DeleteStore(logger); err != nil {
			logger.Error("cleaning-up-store-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		return nil
	},
}
