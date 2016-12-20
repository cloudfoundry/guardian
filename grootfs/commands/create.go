package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	unpackerpkg "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/fetcher/local"
	"code.cloudfoundry.org/grootfs/fetcher/remote"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	storepkg "code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/cache_driver"
	"code.cloudfoundry.org/grootfs/store/dependency_manager"
	imageClonerpkg "code.cloudfoundry.org/grootfs/store/image_cloner"
	locksmithpkg "code.cloudfoundry.org/grootfs/store/locksmith"
	"code.cloudfoundry.org/grootfs/store/volume_driver"
	"code.cloudfoundry.org/lager"

	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	"github.com/docker/distribution/registry/api/errcode"
	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli"
)

var CreateCommand = cli.Command{
	Name:        "create",
	Usage:       "create [options] <image> <id>",
	Description: "Creates a root filesystem for the provided image.",

	Flags: []cli.Flag{
		cli.Int64Flag{
			Name:  "disk-limit-size-bytes",
			Usage: "Inclusive disk limit (i.e: includes all layers in the filesystem)",
		},
		cli.StringSliceFlag{
			Name:  "uid-mapping",
			Usage: "UID mapping for image translation, e.g.: <Namespace UID>:<Host UID>:<Size>",
		},
		cli.StringSliceFlag{
			Name:  "gid-mapping",
			Usage: "GID mapping for image translation, e.g.: <Namespace UID>:<Host UID>:<Size>",
		},
		cli.StringSliceFlag{
			Name:  "insecure-registry",
			Usage: "Whitelist a private registry",
		},
		cli.BoolFlag{
			Name:  "exclude-image-from-quota",
			Usage: "Set disk limit to be exclusive (i.e.: excluding image layers)",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("create")

		if ctx.NArg() != 2 {
			logger.Error("parsing-command", errors.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return cli.NewExitError(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		configBuilder.WithInsecureRegistries(ctx.StringSlice("insecure-registry")).
			WithUIDMappings(ctx.StringSlice("uid-mapping")).
			WithGIDMappings(ctx.StringSlice("gid-mapping"))

		configBuilder.WithDiskLimitSizeBytes(ctx.Int64("disk-limit-size-bytes"),
			ctx.IsSet("disk-limit-size-bytes"))

		configBuilder.WithExcludeBaseImageFromQuota(ctx.Bool("exclude-image-from-quota"),
			ctx.IsSet("exclude-image-from-quota"))

		cfg, err := configBuilder.Build()
		logger.Debug("create-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		storePath := cfg.UserBasedStorePath
		baseImage := ctx.Args().First()
		id := ctx.Args().Tail()[0]

		uidMappings, err := parseIDMappings(cfg.UIDMappings)
		if err != nil {
			err = fmt.Errorf("parsing uid-mapping: %s", err)
			logger.Error("parsing-command", err)
			return cli.NewExitError(err.Error(), 1)
		}
		gidMappings, err := parseIDMappings(cfg.GIDMappings)
		if err != nil {
			err = fmt.Errorf("parsing gid-mapping: %s", err)
			logger.Error("parsing-command", err)
			return cli.NewExitError(err.Error(), 1)
		}

		btrfsVolumeDriver := volume_driver.NewBtrfs(cfg.BtrfsBin, cfg.DraxBin, storePath)
		imageCloner := imageClonerpkg.NewImageCloner(btrfsVolumeDriver, storePath)

		runner := linux_command_runner.New()
		idMapper := unpackerpkg.NewIDMapper(unpackerpkg.SystemIDMappers{
			UID: ctx.GlobalString("newuidmap-bin"),
			GID: ctx.GlobalString("newgidmap-bin"),
		}, runner)
		namespacedCmdUnpacker := unpackerpkg.NewNamespacedUnpacker(runner, idMapper)

		dockerSrc := remote.NewDockerSource(cfg.InsecureRegistries)

		cacheDriver := cache_driver.NewCacheDriver(storePath)
		remoteFetcher := remote.NewRemoteFetcher(dockerSrc, cacheDriver)

		localFetcher := local.NewLocalFetcher()

		locksmith := locksmithpkg.NewFileSystem(storePath)
		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(storePath, storepkg.META_DIR_NAME, "dependencies"),
		)
		baseImagePuller := base_image_puller.NewBaseImagePuller(
			localFetcher, remoteFetcher, namespacedCmdUnpacker, btrfsVolumeDriver,
			dependencyManager,
		)
		rootFSConfigurer := storepkg.NewRootFSConfigurer()
		metricsEmitter := metrics.NewEmitter()
		creator := groot.IamCreator(
			imageCloner, baseImagePuller, locksmith, rootFSConfigurer,
			dependencyManager, metricsEmitter,
		)

		createSpec := groot.CreateSpec{
			ID:                        id,
			BaseImage:                 baseImage,
			DiskLimit:                 cfg.DiskLimitSizeBytes,
			ExcludeBaseImageFromQuota: cfg.ExcludeBaseImageFromQuota,
			UIDMappings:               uidMappings,
			GIDMappings:               gidMappings,
		}
		image, err := creator.Create(logger, createSpec)
		if err != nil {
			logger.Error("creating", err)

			humanizedError := tryHumanize(err, createSpec)
			return cli.NewExitError(humanizedError, 1)
		}

		fmt.Println(image.Path)
		return nil
	},
}

func parseIDMappings(args []string) ([]groot.IDMappingSpec, error) {
	mappings := []groot.IDMappingSpec{}

	for _, v := range args {
		var mapping groot.IDMappingSpec
		_, err := fmt.Sscanf(v, "%d:%d:%d", &mapping.NamespaceID, &mapping.HostID, &mapping.Size)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}

func containsDockerError(errorsList errcode.Errors, errCode errcode.ErrorCode) bool {
	for _, err := range errorsList {
		if e, ok := err.(errcode.Error); ok && e.ErrorCode() == errCode {
			return true
		}
	}

	return false
}

func tryHumanizeDockerErrorsList(err errcode.Errors, spec groot.CreateSpec) string {
	if containsDockerError(err, errcode.ErrorCodeUnauthorized) {
		return fmt.Sprintf("%s does not exist or you do not have permissions to see it.", spec.BaseImage)
	}

	return err.Error()
}

func tryHumanize(err error, spec groot.CreateSpec) string {
	switch e := errorspkg.Cause(err).(type) {
	case *url.Error:
		if _, ok := e.Err.(x509.UnknownAuthorityError); ok {
			return "This registry is insecure. To pull images from this registry, please use the --insecure-registry option."
		}

	case errcode.Errors:
		return tryHumanizeDockerErrorsList(e, spec)
	}

	return errorspkg.Cause(err).Error()
}
