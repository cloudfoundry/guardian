package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/config"
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
			err := errorspkg.Errorf("Store %s can only be initialized by Root user", storePath)
			logger.Error("init-store-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		stat, err := os.Stat(storePath)
		if err == nil && stat.IsDir() {
			err = errorspkg.Errorf("Store already initialized at path %s", storePath)
			logger.Error("init-store-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fsDriver, err := createFileSystemDriver(cfg)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		if err := fsDriver.ValidateFileSystem(logger, filepath.Dir(storePath)); err != nil {
			logger.Error("init-store-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		if err := os.MkdirAll(storePath, 0755); err != nil {
			err := errorspkg.Wrapf(err, "Failed to initialize store path %s", storePath)
			logger.Error("init-store-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}
		return nil
	},
}
