package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"os"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/store/filesystems/loopback"
	"code.cloudfoundry.org/grootfs/store/filesystems/mount"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/lager"
	"github.com/urfave/cli/v2"
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

		rootless := os.Getuid() != 0
		var unmounter overlayxfs.Unmounter = mount.RootfulUnmounter{}
		if rootless {
			unmounter = mount.RootlessUnmounter{}
		}
		fsDriver := overlayxfs.NewDriver(cfg.StorePath, cfg.TardisBin, unmounter, loopback.NewNoopDirectIO())

		storePath := cfg.StorePath
		manager := manager.New(storePath, nil, fsDriver, fsDriver, fsDriver, nil)

		if err := manager.DeleteStore(logger); err != nil {
			logger.Error("cleaning-up-store-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		return nil
	},
}
