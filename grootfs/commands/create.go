package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	unpackerpkg "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/commands/commandrunner"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/fetcher/local"
	"code.cloudfoundry.org/grootfs/fetcher/remote"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	storepkg "code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/cache_driver"
	"code.cloudfoundry.org/grootfs/store/dependency_manager"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/garbage_collector"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	locksmithpkg "code.cloudfoundry.org/grootfs/store/locksmith"
	"code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/lager"

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
			Usage: "GID mapping for image translation, e.g.: <Namespace GID>:<Host GID>:<Size>",
		},
		cli.StringSliceFlag{
			Name:  "insecure-registry",
			Usage: "Whitelist a private registry",
		},
		cli.BoolFlag{
			Name:  "exclude-image-from-quota",
			Usage: "Set disk limit to be exclusive (i.e.: excluding image layers)",
		},
		cli.BoolFlag{
			Name:  "with-clean",
			Usage: "Clean up unused layers before creating rootfs",
		},
		cli.BoolFlag{
			Name:  "without-clean",
			Usage: "Do NOT clean up unused layers before creating rootfs",
		},
		cli.BoolFlag{
			Name:  "with-mount",
			Usage: "Mount the root filesystem after creation. This may require root privileges.",
		},
		cli.BoolFlag{
			Name:  "without-mount",
			Usage: "Do not mount the root filesystem.",
		},
		cli.StringFlag{
			Name:  "username",
			Usage: "Username to authenticate in image registry",
		},
		cli.StringFlag{
			Name:  "password",
			Usage: "Password to authenticate in image registry",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("create")
		newExitError := newErrorHandler(logger, "create")

		if ctx.NArg() != 2 {
			logger.Error("parsing-command", errorspkg.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return newExitError(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		configBuilder.WithInsecureRegistries(ctx.StringSlice("insecure-registry")).
			WithUIDMappings(ctx.StringSlice("uid-mapping")).
			WithGIDMappings(ctx.StringSlice("gid-mapping")).
			WithDiskLimitSizeBytes(ctx.Int64("disk-limit-size-bytes"),
				ctx.IsSet("disk-limit-size-bytes")).
			WithExcludeImageFromQuota(ctx.Bool("exclude-image-from-quota"),
				ctx.IsSet("exclude-image-from-quota")).
			WithClean(ctx.IsSet("with-clean"), ctx.IsSet("without-clean")).
			WithMount(ctx.IsSet("with-mount"), ctx.IsSet("without-mount"))

		cfg, err := configBuilder.Build()
		logger.Debug("create-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return newExitError(err.Error(), 1)
		}

		if err := validateOptions(ctx, cfg); err != nil {
			return newExitError(err.Error(), 1)
		}

		storePath := cfg.StorePath
		baseImage := ctx.Args().First()
		id := ctx.Args().Tail()[0]

		uidMappings, err := parseIDMappings(cfg.Create.UIDMappings)
		if err != nil {
			err = errorspkg.Errorf("parsing uid-mapping: %s", err)
			logger.Error("parsing-command", err)
			return newExitError(err.Error(), 1)
		}
		gidMappings, err := parseIDMappings(cfg.Create.GIDMappings)
		if err != nil {
			err = errorspkg.Errorf("parsing gid-mapping: %s", err)
			logger.Error("parsing-command", err)
			return newExitError(err.Error(), 1)
		}

		storeOwnerUid, storeOwnerGid, err := findStoreOwner(uidMappings, gidMappings)
		if err != nil {
			return err
		}

		fsDriver, err := createFileSystemDriver(cfg)
		if err != nil {
			return newExitError(err.Error(), 1)
		}

		metricsEmitter := metrics.NewEmitter()
		var globalLocksmith groot.Locksmith
		var pullerLocksmith groot.Locksmith

		if ctx.GlobalBool("lockless") {
			globalLocksmith = &locksmithpkg.Null{}
			pullerLocksmith = locksmithpkg.NewFileSystem(storePath, metricsEmitter)
		} else {
			globalLocksmith = locksmithpkg.NewFileSystem(storePath, metricsEmitter)
			pullerLocksmith = &locksmithpkg.Null{}
		}
		manager := manager.New(storePath, globalLocksmith, fsDriver, fsDriver, fsDriver)
		if err = manager.ConfigureStore(logger, storeOwnerUid, storeOwnerGid); err != nil {
			exitErr := errorspkg.Wrapf(errorspkg.Cause(err), "Image id '%s'", id)
			logger.Error("failed-to-setup-store", err, lager.Data{"id": id})
			return newExitError(exitErr.Error(), 1)
		}

		imageCloner := image_cloner.NewImageCloner(fsDriver, storePath)

		runner := commandrunner.New()
		var unpacker base_image_puller.Unpacker
		unpackerStrategy := unpackerpkg.UnpackStrategy{
			Name:               cfg.FSDriver,
			WhiteoutDevicePath: filepath.Join(storePath, overlayxfs.WhiteoutDevice),
		}
		if os.Getuid() == 0 {
			unpacker = unpackerpkg.NewTarUnpacker(unpackerStrategy)
		} else {
			idMapper := unpackerpkg.NewIDMapper(cfg.NewuidmapBin, cfg.NewgidmapBin, runner)
			unpacker = unpackerpkg.NewNSIdMapperUnpacker(runner, idMapper, unpackerStrategy)
		}

		dockerSrc := remote.NewDockerSource(ctx.String("username"), ctx.String("password"), cfg.Create.InsecureRegistries)

		cacheDriver := cache_driver.NewCacheDriver(storePath)
		remoteFetcher := remote.NewRemoteFetcher(dockerSrc, cacheDriver)

		localFetcher := local.NewLocalFetcher()

		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(storePath, storepkg.MetaDirName, "dependencies"),
		)
		baseImagePuller := base_image_puller.NewBaseImagePuller(
			localFetcher,
			remoteFetcher,
			unpacker,
			fsDriver,
			dependencyManager,
			metricsEmitter,
			pullerLocksmith,
		)

		sm := storepkg.NewStoreMeasurer(storePath)
		gc := garbage_collector.NewGC(cacheDriver, fsDriver, imageCloner, dependencyManager)
		cleaner := groot.IamCleaner(globalLocksmith, sm, gc, metricsEmitter)

		namespaceChecker := groot.NewStoreNamespacer(storePath)

		rootFSConfigurer := storepkg.NewRootFSConfigurer()
		creator := groot.IamCreator(
			imageCloner, baseImagePuller, globalLocksmith, rootFSConfigurer,
			dependencyManager, metricsEmitter, cleaner, namespaceChecker,
		)

		createSpec := groot.CreateSpec{
			ID:                          id,
			Mount:                       !cfg.Create.WithoutMount,
			BaseImage:                   baseImage,
			DiskLimit:                   cfg.Create.DiskLimitSizeBytes,
			ExcludeBaseImageFromQuota:   cfg.Create.ExcludeImageFromQuota,
			UIDMappings:                 uidMappings,
			GIDMappings:                 gidMappings,
			CleanOnCreate:               cfg.Create.WithClean,
			CleanOnCreateThresholdBytes: cfg.Clean.ThresholdBytes,
			CleanOnCreateIgnoreImages:   cfg.Clean.IgnoreBaseImages,
		}
		image, err := creator.Create(logger, createSpec)
		if err != nil {
			logger.Error("creating", err)
			humanizedError := tryHumanize(err, createSpec)
			return newExitError(humanizedError, 1)
		}

		jsonBytes, err := json.Marshal(image)
		if err != nil {
			logger.Error("formatting output", err)
			return newExitError(err.Error(), 1)
		}
		fmt.Println(string(jsonBytes))

		usage, err := sm.MeasureStore(logger)
		if err != nil {
			logger.Error("measuring-store", err)
			return newExitError(err.Error(), 1)
		}

		metricsEmitter.TryIncrementRunCount("create", nil)
		metricsEmitter.TryEmitUsage(logger, "StoreUsage", usage)

		return nil
	},
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

func tryParsingErrorMessage(err error) error {
	newErr := errorspkg.Cause(err)
	if newErr.Error() == "unable to retrieve auth token: 401 unauthorized" {
		return errorspkg.New("authorization failed: username and password are invalid")
	}
	if newErr.Error() == "directory provided instead of a tar file" {
		return errorspkg.New("invalid base image: " + newErr.Error())

	}

	return err
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

	return tryParsingErrorMessage(err).Error()
}

func findStoreOwner(uidMappings, gidMappings []groot.IDMappingSpec) (int, int, error) {
	uid := os.Getuid()
	gid := os.Getgid()

	for _, mapping := range uidMappings {
		if mapping.Size == 1 && mapping.NamespaceID == 0 {
			uid = mapping.HostID
			break
		}
		uid = -1
	}

	if len(uidMappings) > 0 && uid == -1 {
		return 0, 0, errorspkg.New("couldn't determine store owner, missing root user mapping")
	}

	for _, mapping := range gidMappings {
		if mapping.Size == 1 && mapping.NamespaceID == 0 {
			gid = mapping.HostID
			break
		}
		gid = -1
	}

	if len(gidMappings) > 0 && gid == -1 {
		return 0, 0, errorspkg.New("couldn't determine store owner, missing root user mapping")
	}

	return uid, gid, nil
}

func validateOptions(ctx *cli.Context, cfg config.Config) error {
	if ctx.IsSet("with-clean") && ctx.IsSet("without-clean") {
		return errorspkg.New("with-clean and without-clean cannot be used together")
	}

	return nil
}
