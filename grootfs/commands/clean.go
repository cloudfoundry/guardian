package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"fmt"
	"path/filepath"

	"code.cloudfoundry.org/lager"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	storepkg "code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/cache_driver"
	"code.cloudfoundry.org/grootfs/store/dependency_manager"
	"code.cloudfoundry.org/grootfs/store/garbage_collector"
	imageClonerpkg "code.cloudfoundry.org/grootfs/store/image_cloner"
	locksmithpkg "code.cloudfoundry.org/grootfs/store/locksmith"
	"code.cloudfoundry.org/grootfs/store/volume_driver"

	"github.com/urfave/cli"
)

var CleanCommand = cli.Command{
	Name:        "clean",
	Usage:       "clean",
	Description: "Cleans up unused layers",

	Flags: []cli.Flag{
		cli.Uint64Flag{
			Name:  "threshold-bytes",
			Usage: "Disk usage of the store directory at which cleanup should trigger.",
		},
		cli.StringSliceFlag{
			Name:  "ignore-image",
			Usage: "Images to ignore during cleanup",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("clean")

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		configBuilder.WithIgnoreBaseImages(ctx.StringSlice("ignore-image"))
		configBuilder.WithCleanThresholdBytes(ctx.Uint64("threshold-bytes"),
			ctx.IsSet("threshold-bytes"))

		cfg, err := configBuilder.Build()
		logger.Debug("clean-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		storePath := cfg.UserBasedStorePath

		btrfsVolumeDriver := volume_driver.NewBtrfs(cfg.BtrfsBin, cfg.DraxBin, storePath)
		imageCloner := imageClonerpkg.NewImageCloner(btrfsVolumeDriver, storePath)
		locksmith := locksmithpkg.NewFileSystem(storePath)
		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(storePath, storepkg.META_DIR_NAME, "dependencies"),
		)
		cacheDriver := cache_driver.NewCacheDriver(storePath)
		sm := garbage_collector.NewStoreMeasurer(storePath)
		gc := garbage_collector.NewGC(cacheDriver, btrfsVolumeDriver, imageCloner, dependencyManager)

		metricsEmitter := metrics.NewEmitter()
		cleaner := groot.IamCleaner(locksmith, sm, gc, metricsEmitter)

		noop, err := cleaner.Clean(logger, cfg.CleanThresholdBytes, cfg.IgnoreBaseImages)
		if err != nil {
			logger.Error("cleaning-up-unused-resources", err)
			return cli.NewExitError(err.Error(), 1)
		}

		if noop {
			fmt.Println("threshold not reached: skipping clean")
			return nil
		}

		fmt.Println("clean completed")
		return nil
	},
}
