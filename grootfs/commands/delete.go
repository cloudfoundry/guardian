package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/commands/idfinder"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/dependency_manager"
	"code.cloudfoundry.org/grootfs/store/filesystems/mount"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/garbage_collector"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
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
			logger.Error("parsing-command", errorspkg.New("id was not specified"))
			return cli.NewExitError("id was not specified", 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		cfg, err := configBuilder.Build()
		logger.Debug("delete-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		storePath := cfg.StorePath
		idOrPath := ctx.Args().First()
		id, err := idfinder.FindID(storePath, idOrPath)
		if err != nil {
			logger.Debug("id-not-found-skipping", lager.Data{"id": idOrPath, "storePath": storePath, "errorMessage": err.Error()})
			fmt.Printf("%s Skipping delete.\n", err)
			return nil
		}

		rootless := os.Getuid() != 0
		var unmounter overlayxfs.Unmounter = mount.RootfulUnmounter{}
		if rootless {
			unmounter = mount.RootlessUnmounter{}
		}
		fsDriver := overlayxfs.NewDriver(cfg.StorePath, cfg.TardisBin, unmounter)

		imageDriver, err := createImageDriver(logger, cfg, fsDriver)
		if err != nil {
			logger.Error("failed-to-initialise-image-driver", err)
			return cli.NewExitError(err.Error(), 1)
		}

		imageCloner := image_cloner.NewImageCloner(imageDriver, storePath)
		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(storePath, store.MetaDirName, "dependencies"),
		)

		metricsEmitter := metrics.NewEmitter(logger, cfg.MetronEndpoint)
		deleter := groot.IamDeleter(imageCloner, dependencyManager, metricsEmitter)

		gc := garbage_collector.NewGC(fsDriver, imageCloner, dependencyManager)
		sm := store.NewStoreMeasurer(storePath, fsDriver, gc)

		defer func() {
			unusedVolumesSize, err := sm.UnusedVolumesSize(logger)
			if err != nil {
				logger.Error("getting-unused-layers-size", err)
			}
			metricsEmitter.TryEmitUsage(logger, "UnusedLayersSize", unusedVolumesSize, "bytes")
		}()

		err = deleter.Delete(logger, id)
		if err != nil {
			logger.Error("deleting-image-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fmt.Printf("Image %s deleted\n", id)
		return nil
	},
}
