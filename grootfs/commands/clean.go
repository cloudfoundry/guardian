package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	storepkg "code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/dependency_manager"
	"code.cloudfoundry.org/grootfs/store/filesystems/loopback"
	"code.cloudfoundry.org/grootfs/store/filesystems/mount"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/garbage_collector"
	imagemanagerpkg "code.cloudfoundry.org/grootfs/store/image_manager"
	locksmithpkg "code.cloudfoundry.org/grootfs/store/locksmith"
	errorspkg "github.com/pkg/errors"

	"github.com/urfave/cli/v2"
)

var CleanCommand = cli.Command{
	Name:        "clean",
	Usage:       "clean",
	Description: "Cleans up unused layers",

	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:  "threshold-bytes",
			Usage: "Disk usage of the store directory at which cleanup should trigger",
		},
	},

	Action: func(ctx *cli.Context) (exitError error) {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("clean")

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		configBuilder.WithCleanThresholdBytes(ctx.Int64("threshold-bytes"),
			ctx.IsSet("threshold-bytes"))

		cfg, err := configBuilder.Build()
		if err != nil {
			fmt.Fprintf(os.Stderr, "config-builder-failed: %v", err)
			return cli.NewExitError(err.Error(), 1)
		}

		if _, err = os.Stat(cfg.StorePath); os.IsNotExist(err) {
			err = errorspkg.Errorf("no store found at %s", cfg.StorePath)
			fmt.Fprintf(os.Stderr, "no store found at %s", cfg.StorePath)
			return cli.NewExitError(err.Error(), 0)
		}

		metricsEmitter := metrics.NewEmitter(logger, cfg.MetronEndpoint)
		locksDir := filepath.Join(cfg.StorePath, storepkg.LocksDirName)
		locksmith := locksmithpkg.NewExclusiveFileSystem(locksDir).WithMetrics(metricsEmitter)
		lockFile, err := locksmith.Lock(groot.GCLockKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to acquire lock %s: %v", groot.GCLockKey, err)
			return cli.NewExitError(err.Error(), 1)
		}
		defer func() {
			if err := locksmith.Unlock(lockFile); err != nil {
				logger.Error("release-lock-failed", err, nil)
				exitError = cli.NewExitError(err.Error(), 1)
			}
		}()

		logger.Debug("clean-config", lager.Data{"currentConfig": cfg})

		rootless := os.Getuid() != 0
		var unmounter overlayxfs.Unmounter = mount.RootfulUnmounter{}
		if rootless {
			unmounter = mount.RootlessUnmounter{}
		}
		fsDriver := overlayxfs.NewDriver(cfg.StorePath, cfg.TardisBin, unmounter, loopback.NewNoopDirectIO())

		imageManager := imagemanagerpkg.NewImageManager(fsDriver, cfg.StorePath)
		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(cfg.StorePath, storepkg.MetaDirName, "dependencies"),
		)

		nsFsDriver, err := createImageDriver(logger, cfg, fsDriver)
		if err != nil {
			logger.Error("failed-to-create-image-driver", err)
			return cli.NewExitError(err.Error(), 1)
		}
		gc := garbage_collector.NewGC(nsFsDriver, imageManager, dependencyManager)
		sm := storepkg.NewStoreMeasurer(cfg.StorePath, fsDriver, gc)

		cleaner := groot.IamCleaner(locksmith, sm, gc, metricsEmitter)

		defer func() {
			unusedVolumesSize, err := sm.UnusedVolumesSize(logger)
			if err != nil {
				logger.Error("getting-unused-volumes-size", err)
			}
			metricsEmitter.TryEmitUsage(logger, "UnusedLayersSize", unusedVolumesSize, "bytes")

			usedVolumesSize, err := sm.UsedVolumesSize(logger)
			if err != nil {
				logger.Error("getting-used-volumes-size", err)
			}
			logger.Info("usedLayersSize value ", UsedVolumesSize)
			metricsEmitter.TryEmitUsage(logger, "UsedLayersSize", usedVolumesSize, "bytes")
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

		return nil
	},
}
