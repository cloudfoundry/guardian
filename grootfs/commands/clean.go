package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	"code.cloudfoundry.org/lager"

	unpackerpkg "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	storepkg "code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/dependency_manager"
	"code.cloudfoundry.org/grootfs/store/filesystems/namespaced"
	"code.cloudfoundry.org/grootfs/store/garbage_collector"
	imageClonerpkg "code.cloudfoundry.org/grootfs/store/image_cloner"
	locksmithpkg "code.cloudfoundry.org/grootfs/store/locksmith"
	errorspkg "github.com/pkg/errors"

	"github.com/urfave/cli"
)

var CleanCommand = cli.Command{
	Name:        "clean",
	Usage:       "clean",
	Description: "Cleans up unused layers",

	Flags: []cli.Flag{
		cli.Int64Flag{
			Name:  "threshold-bytes",
			Usage: "Disk usage of the store directory at which cleanup should trigger",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("clean")

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		configBuilder.WithCleanThresholdBytes(ctx.Int64("threshold-bytes"),
			ctx.IsSet("threshold-bytes"))

		cfg, err := configBuilder.Build()
		logger.Debug("clean-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		storePath := cfg.StorePath
		if _, err = os.Stat(storePath); os.IsNotExist(err) {
			err = errorspkg.Errorf("no store found at %s", storePath)
			logger.Error("store-path-failed", err, nil)
			return cli.NewExitError(err.Error(), 0)
		}

		fsDriver, err := createFileSystemDriver(cfg)
		if err != nil {
			logger.Error("failed-to-initialise-filesystem-driver", err)
			return cli.NewExitError(err.Error(), 1)
		}

		imageCloner := imageClonerpkg.NewImageCloner(fsDriver, storePath)
		metricsEmitter := metrics.NewEmitter(logger, cfg.MetronEndpoint)

		locksmith := locksmithpkg.NewExclusiveFileSystem(storePath, metricsEmitter)
		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(storePath, storepkg.MetaDirName, "dependencies"),
		)

		storeNamespacer := groot.NewStoreNamespacer(storePath)
		idMappings, err := storeNamespacer.Read()
		if err != nil {
			logger.Error("reading-namespace-file", err)
			return cli.NewExitError(err.Error(), 1)
		}

		runner := linux_command_runner.New()
		idMapper := unpackerpkg.NewIDMapper(cfg.NewuidmapBin, cfg.NewgidmapBin, runner)
		nsFsDriver := namespaced.New(fsDriver, idMappings, idMapper, runner)
		gc := garbage_collector.NewGC(nsFsDriver, imageCloner, dependencyManager)
		sm := storepkg.NewStoreMeasurer(storePath, fsDriver, gc)

		cleaner := groot.IamCleaner(locksmith, sm, gc, metricsEmitter)

		defer func() {
			unusedVolumesSize, err := sm.UnusedVolumesSize(logger)
			if err != nil {
				logger.Error("getting-unused-volumes-size", err)
			}
			metricsEmitter.TryEmitUsage(logger, "UnusedLayersSize", unusedVolumesSize, "bytes")
		}()

		noop, err := cleaner.Clean(logger, cfg.Clean.ThresholdBytes)
		if err != nil {
			logger.Error("cleaning-up-unused-resources", err)
			return cli.NewExitError(err.Error(), 1)
		}

		if noop {
			fmt.Println("threshold not reached: skipping clean")
			return nil
		}

		fmt.Println("clean completed")

		usage, err := sm.Usage(logger)
		if err != nil {
			logger.Error("measuring-store", err)
			return cli.NewExitError(err.Error(), 1)
		}

		metricsEmitter.TryEmitUsage(logger, "StoreUsage", usage, "bytes")
		return nil
	},
}
