package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"fmt"
	"path/filepath"

	"code.cloudfoundry.org/lager"

	"code.cloudfoundry.org/grootfs/commands/storepath"
	"code.cloudfoundry.org/grootfs/groot"
	storepkg "code.cloudfoundry.org/grootfs/store"
	bundlerpkg "code.cloudfoundry.org/grootfs/store/bundler"
	"code.cloudfoundry.org/grootfs/store/cache_driver"
	"code.cloudfoundry.org/grootfs/store/dependency_manager"
	"code.cloudfoundry.org/grootfs/store/garbage_collector"
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
	},

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("clean")

		storePath, err := storepath.UserBased(ctx.GlobalString("store"))
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("can't determine the store path: %s", err.Error()), 1)
		}

		btrfsVolumeDriver := volume_driver.NewBtrfs(ctx.GlobalString("drax-bin"), storePath)
		bundler := bundlerpkg.NewBundler(btrfsVolumeDriver, storePath)
		locksmith := locksmithpkg.NewFileSystem(storePath)
		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(storePath, storepkg.META_DIR_NAME, "dependencies"),
		)
		cacheDriver := cache_driver.NewCacheDriver(storePath)
		sm := garbage_collector.NewStoreMeasurer(storePath)
		gc := garbage_collector.NewGC(cacheDriver, btrfsVolumeDriver, bundler, dependencyManager)

		cleaner := groot.IamCleaner(locksmith, sm, gc)
		err = cleaner.Clean(logger, ctx.Uint64("threshold-bytes"))
		if err != nil {
			logger.Error("cleaning-up-unused-resources", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fmt.Println("Clean finished")
		return nil
	},
}
