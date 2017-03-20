package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"os"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/store/locksmith"
	"code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/lager"

	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli"
)

var InitStoreCommand = cli.Command{
	Name:        "init-store",
	Usage:       "init-store",
	Description: "Initialize a Store Directory on a mounted Filesystem",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("init-store")

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		cfg, err := configBuilder.Build()
		logger.Debug("init-store", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		storePath := cfg.StorePath

		if os.Getuid() != 0 {
			err := errorspkg.Errorf("store %s can only be initialized by Root user", storePath)
			logger.Error("init-store-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fsDriver, err := createFileSystemDriver(cfg)
		if err != nil {
			logger.Error("failed-to-initialise-driver", err)
			return cli.NewExitError(err.Error(), 1)
		}

		locksmith := locksmith.NewFileSystem(storePath)
		manager := manager.New(storePath, locksmith, fsDriver, fsDriver, fsDriver)
		if err := manager.InitStore(logger); err != nil {
			logger.Error("cleaning-up-store-failed", err)
			return cli.NewExitError(errorspkg.Cause(err).Error(), 1)
		}

		return nil
	},
}
