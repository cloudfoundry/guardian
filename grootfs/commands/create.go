package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	"code.cloudfoundry.org/grootfs/base_image_puller"
	unpackerpkg "code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"
	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher/source"
	"code.cloudfoundry.org/grootfs/fetcher/tar_fetcher"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	storepkg "code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/dependency_manager"
	"code.cloudfoundry.org/grootfs/store/filesystems/namespaced"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/garbage_collector"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	locksmithpkg "code.cloudfoundry.org/grootfs/store/locksmith"
	"code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/lager"

	"github.com/containers/image/types"
	"github.com/docker/distribution/registry/api/errcode"
	specs "github.com/opencontainers/runtime-spec/specs-go"
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
			Name:  "insecure-registry",
			Usage: "Whitelist a private registry",
		},
		cli.BoolFlag{
			Name:  "exclude-image-from-quota",
			Usage: "Set disk limit to be exclusive (i.e.: excluding image layers)",
		},
		cli.BoolFlag{
			Name:  "skip-layer-validation",
			Usage: "Do not validate checksums and sizes of image layers. (Can only be used with oci:/// protocol images.)",
		},
		cli.BoolFlag{
			Name:  "with-clean",
			Usage: "Clean up unused layers before creating rootfs",
		},
		cli.BoolFlag{
			Name:  "without-clean",
			Usage: "Do NOT clean up unused layers before creating rootfs",
		},
		cli.Int64Flag{
			Name:  "threshold-bytes",
			Usage: "Disk usage of the store directory at which cleanup should trigger",
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

		if ctx.NArg() != 2 {
			logger.Error("parsing-command", errorspkg.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return cli.NewExitError(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		configBuilder.WithInsecureRegistries(ctx.StringSlice("insecure-registry")).
			WithDiskLimitSizeBytes(ctx.Int64("disk-limit-size-bytes"),
				ctx.IsSet("disk-limit-size-bytes")).
			WithExcludeImageFromQuota(ctx.Bool("exclude-image-from-quota"),
				ctx.IsSet("exclude-image-from-quota")).
			WithSkipLayerValidation(ctx.Bool("skip-layer-validation"),
				ctx.IsSet("skip-layer-validation")).
			WithCleanThresholdBytes(ctx.Int64("threshold-bytes"), ctx.IsSet("threshold-bytes")).
			WithClean(ctx.IsSet("with-clean"), ctx.IsSet("without-clean")).
			WithMount(ctx.IsSet("with-mount"), ctx.IsSet("without-mount"))

		cfg, err := configBuilder.Build()
		logger.Debug("create-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		if err = validateOptions(ctx, cfg); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		storePath := cfg.StorePath
		id := ctx.Args().Tail()[0]
		baseImage := ctx.Args().First()
		baseImageURL, err := url.Parse(baseImage)
		if err != nil {
			logger.Error("base-image-url-parsing-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fsDriver, err := createFileSystemDriver(cfg)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		metricsEmitter := metrics.NewEmitter(logger, cfg.MetronEndpoint)

		initLocksDir := filepath.Join("/", "var", "run")
		storeLocksDir := filepath.Join(storePath, storepkg.LocksDirName)
		sharedLocksmith := locksmithpkg.NewSharedFileSystem(storeLocksDir).WithMetrics(metricsEmitter)
		exclusiveLocksmith := locksmithpkg.NewExclusiveFileSystem(storeLocksDir).WithMetrics(metricsEmitter)
		initStoreLocksmith := locksmithpkg.NewExclusiveFileSystem(initLocksDir)

		imageCloner := image_cloner.NewImageCloner(fsDriver, storePath)
		storeNamespacer := groot.NewStoreNamespacer(storePath)

		manager := manager.New(storePath, storeNamespacer, fsDriver, fsDriver, fsDriver, initStoreLocksmith)
		if !manager.IsStoreInitialized(logger) {
			logger.Error("store-verification-failed", errors.New("store is not initialized"))
			return cli.NewExitError("Store path is not initialized. Please run init-store.", 1)
		}

		idMappings, err := storeNamespacer.Read()
		if err != nil {
			logger.Error("reading-namespace-file", err)
			return cli.NewExitError(err.Error(), 1)
		}

		runner := linux_command_runner.New()
		var unpacker base_image_puller.Unpacker
		unpackerStrategy := unpackerpkg.UnpackStrategy{
			Name:               cfg.FSDriver,
			WhiteoutDevicePath: filepath.Join(storePath, overlayxfs.WhiteoutDevice),
		}

		var idMapper unpackerpkg.IDMapper
		if os.Getuid() == 0 {
			unpacker, err = unpackerpkg.NewTarUnpacker(unpackerStrategy)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
		} else {
			idMapper = unpackerpkg.NewIDMapper(cfg.NewuidmapBin, cfg.NewgidmapBin, runner)
			unpacker = unpackerpkg.NewNSIdMapperUnpacker(runner, idMapper, unpackerStrategy)
		}

		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(storePath, storepkg.MetaDirName, "dependencies"),
		)

		nsFsDriver := namespaced.New(fsDriver, idMappings, idMapper, runner)

		systemContext := createSystemContext(baseImageURL, cfg.Create, ctx.String("username"), ctx.String("password"))

		fetcher := createFetcher(baseImageURL, systemContext, cfg.Create)
		defer func() {
			err := fetcher.Close()
			if err != nil {
				logger.Error("closing-fetcher", err)
			}
		}()

		baseImagePuller := base_image_puller.NewBaseImagePuller(
			fetcher,
			unpacker,
			nsFsDriver,
			metricsEmitter,
			exclusiveLocksmith,
		)

		gc := garbage_collector.NewGC(nsFsDriver, imageCloner, dependencyManager)
		sm := storepkg.NewStoreMeasurer(storePath, fsDriver, gc)
		cleaner := groot.IamCleaner(exclusiveLocksmith, sm, gc, metricsEmitter)

		creator := groot.IamCreator(
			imageCloner, baseImagePuller, sharedLocksmith,
			dependencyManager, metricsEmitter, cleaner,
		)

		createSpec := groot.CreateSpec{
			ID:                          id,
			Mount:                       !cfg.Create.WithoutMount,
			BaseImageURL:                baseImageURL,
			DiskLimit:                   cfg.Create.DiskLimitSizeBytes,
			ExcludeBaseImageFromQuota:   cfg.Create.ExcludeImageFromQuota,
			UIDMappings:                 idMappings.UIDMappings,
			GIDMappings:                 idMappings.GIDMappings,
			CleanOnCreate:               cfg.Create.WithClean,
			CleanOnCreateThresholdBytes: cfg.Clean.ThresholdBytes,
		}
		image, err := creator.Create(logger, createSpec)
		if err != nil {
			logger.Error("creating", err)
			humanizedError := tryHumanize(err, createSpec)
			return cli.NewExitError(humanizedError, 1)
		}

		containerSpec := specs.Spec{
			Root: &specs.Root{
				Path: image.Rootfs,
			},
			Process: &specs.Process{
				Env: image.Image.Config.Env,
			},
			Mounts: []specs.Mount{},
		}

		for _, mount := range image.Mounts {
			containerSpec.Mounts = append(containerSpec.Mounts, specs.Mount{
				Destination: mount.Destination,
				Type:        mount.Type,
				Source:      mount.Source,
				Options:     mount.Options,
			})
		}

		jsonBytes, err := json.Marshal(containerSpec)
		if err != nil {
			logger.Error("formatting output", err)
			return cli.NewExitError(err.Error(), 1)
		}
		fmt.Println(string(jsonBytes))

		emitMetrics(logger, metricsEmitter, sm)

		return nil
	},
}

func emitMetrics(logger lager.Logger, metricsEmitter *metrics.Emitter, sm *storepkg.StoreMeasurer) {
	usage, err := sm.Usage(logger)
	if err != nil {
		logger.Info(fmt.Sprintf("measuring-store: %s", err))
	}
	metricsEmitter.TryEmitUsage(logger, "StoreUsage", usage, "bytes")

	unusedVolumesSize, err := sm.UnusedVolumesSize(logger)
	if err != nil {
		logger.Info(fmt.Sprintf("getting-unused-layers-size: %s", err))
	}
	metricsEmitter.TryEmitUsage(logger, "UnusedLayersSize", unusedVolumesSize, "bytes")

	totalVolumesSize, err := sm.TotalVolumesSize(logger)
	if err != nil {
		logger.Info(fmt.Sprintf("getting-total-layers-size: %s", err))
	}
	metricsEmitter.TryEmitUsage(logger, "DownloadedLayersSizeInBytes", totalVolumesSize, "bytes")

	commitedQuota, err := sm.CommittedQuota(logger)
	if err != nil {
		logger.Info(fmt.Sprintf("getting-commited-quota: %s", err))
	}
	metricsEmitter.TryEmitUsage(logger, "CommittedQuotaInBytes", commitedQuota, "bytes")
}

func createFetcher(baseImageUrl *url.URL, systemContext types.SystemContext, createCfg config.Create) base_image_puller.Fetcher {
	if baseImageUrl.Scheme == "" {
		return tar_fetcher.NewTarFetcher(baseImageUrl)
	}

	skipOCILayerValidation := createCfg.SkipLayerValidation && baseImageUrl.Scheme == "oci"
	layerSource := source.NewLayerSource(systemContext, skipOCILayerValidation, shouldSkipImageQuotaValidation(createCfg), createCfg.DiskLimitSizeBytes, baseImageUrl)
	return layer_fetcher.NewLayerFetcher(&layerSource)
}

func shouldSkipImageQuotaValidation(createCfg config.Create) bool {
	return createCfg.ExcludeImageFromQuota || createCfg.DiskLimitSizeBytes == 0
}

func createSystemContext(baseImageURL *url.URL, createConfig config.Create, username, password string) types.SystemContext {
	scheme := baseImageURL.Scheme
	switch scheme {
	case "docker":
		return types.SystemContext{
			DockerInsecureSkipTLSVerify: skipTLSValidation(baseImageURL, createConfig.InsecureRegistries),
			DockerAuthConfig: &types.DockerAuthConfig{
				Username: username,
				Password: password,
			},
		}
	case "oci":
		return types.SystemContext{
			OCICertPath: createConfig.RemoteLayerClientCertificatesPath,
		}
	default:
		return types.SystemContext{}
	}

}

func skipTLSValidation(baseImageURL *url.URL, trustedRegistries []string) bool {
	for _, trustedRegistry := range trustedRegistries {
		if baseImageURL.Host == trustedRegistry {
			return true
		}
	}

	return false
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
		return fmt.Sprintf("%s does not exist or you do not have permissions to see it.", spec.BaseImageURL.String())
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
	if regexp.MustCompile("fetching image reference: .*: no such file or directory").MatchString(err.Error()) {
		return errorspkg.New("Image source doesn't exist")
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

func validateOptions(ctx *cli.Context, cfg config.Config) error {
	if ctx.IsSet("with-clean") && ctx.IsSet("without-clean") {
		return errorspkg.New("with-clean and without-clean cannot be used together")
	}

	return nil
}
