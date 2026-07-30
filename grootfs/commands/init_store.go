package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store/filesystems/loopback"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	locksmithpkg "code.cloudfoundry.org/grootfs/store/locksmith"
	"code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/lager/v3"

	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var InitStoreCommand = cli.Command{
	Name:        "init-store",
	Usage:       "init-store --store <path>",
	Description: "Initialize a Store Directory",

	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "uid-mapping",
			Usage: "UID mapping for image translation, e.g.: <Namespace UID>:<Host UID>:<Size>",
		},
		&cli.StringSliceFlag{
			Name:  "gid-mapping",
			Usage: "GID mapping for image translation, e.g.: <Namespace GID>:<Host GID>:<Size>",
		},
		&cli.Int64Flag{
			Name:  "store-size-bytes",
			Usage: "Creates a new filesystem of the given size and mounts it to the given Store Directory",
		},
		&cli.BoolFlag{
			Name:  "with-direct-io",
			Usage: "Enable direct IO on the loopback device associated with the backing store",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("init-store")

		if ctx.NArg() != 0 {
			logger.Error("parsing-command", errorspkg.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return cli.Exit(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		if ctx.IsSet("store-size-bytes") {
			configBuilder = configBuilder.WithStoreSizeBytes(ctx.Int64("store-size-bytes"))
		}
		if ctx.IsSet("with-direct-io") {
			configBuilder = configBuilder.WithDirectIO()
		}

		cfg, err := configBuilder.Build()
		logger.Debug("init-store", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.Exit(err.Error(), 1)
		}

		storePath := cfg.StorePath
		storeSizeBytes := cfg.Init.StoreSizeBytes

		if os.Getuid() != 0 {
			err := errorspkg.Errorf("store %s can only be initialized by Root user", storePath)
			logger.Error("init-store-failed", err)
			return cli.Exit(err.Error(), 1)
		}

		var directIO overlayxfs.DirectIO = loopback.NewDirectIODisabler()
		if cfg.Init.WithDirectIO {
			directIO = loopback.NewDirectIOEnabler()
		}
		fsDriver := overlayxfs.NewDriver(cfg.StorePath, cfg.TardisBin, nil, directIO)

		uidMappings, err := parseIDMappings(ctx.StringSlice("uid-mapping"))
		if err != nil {
			err = errorspkg.Errorf("parsing uid-mapping: %s", err)
			logger.Error("parsing-command", err)
			return cli.Exit(err.Error(), 1)
		}
		gidMappings, err := parseIDMappings(ctx.StringSlice("gid-mapping"))
		if err != nil {
			err = errorspkg.Errorf("parsing gid-mapping: %s", err)
			logger.Error("parsing-command", err)
			return cli.Exit(err.Error(), 1)
		}

		namespacer := groot.NewStoreNamespacer(storePath)
		spec := manager.InitSpec{
			UIDMappings:    uidMappings,
			GIDMappings:    gidMappings,
			StoreSizeBytes: storeSizeBytes,
		}

		initLocksDir := filepath.Join("/", "var", "run")
		initStoreLocksmith := locksmithpkg.NewExclusiveFileSystem(initLocksDir)

		manager := manager.New(storePath, namespacer, fsDriver, fsDriver, fsDriver, initStoreLocksmith)
		if err := manager.InitStore(logger, spec); err != nil {
			logger.Error("init-store-failed", err)
			return cli.Exit(errorspkg.Cause(err).Error(), 1)
		}

		return nil
	},
}
