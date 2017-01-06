package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"errors"
	"fmt"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/commands/idfinder"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/dependency_manager"
	imageClonerpkg "code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/grootfs/store/volume_driver"
	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"
)

var DeleteCommand = cli.Command{
	Name:        "delete",
	Usage:       "delete <id|image path>",
	Description: "Deletes a container image",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("delete")

		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errors.New("id was not specified"))
			return cli.NewExitError("id was not specified", 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		cfg, err := configBuilder.Build()
		logger.Debug("delete-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		storePath := cfg.BaseStorePath
		idOrPath := ctx.Args().First()
		id, err := idfinder.FindID(storePath, idOrPath)
		if err != nil {
			logger.Error("find-id-failed", err, lager.Data{"id": idOrPath, "storePath": storePath})
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

		btrfsVolumeDriver := volume_driver.NewBtrfs(cfg.BtrfsBin, cfg.DraxBin, storePath)
		imageCloner := imageClonerpkg.NewImageCloner(btrfsVolumeDriver, storePath)
		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(storePath, store.META_DIR_NAME, "dependencies"),
		)
		metricsEmitter := metrics.NewEmitter()
		deleter := groot.IamDeleter(imageCloner, dependencyManager, metricsEmitter)

		err = deleter.Delete(logger, id)
		if err != nil {
			logger.Error("deleting-image-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fmt.Printf("Image %s deleted\n", id)
		return nil
	},
}
