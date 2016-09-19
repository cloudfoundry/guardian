package commands

import (
	"errors"
	"fmt"
	"net/url"

	"code.cloudfoundry.org/grootfs/fetcher/local"
	"code.cloudfoundry.org/grootfs/fetcher/remote"
	grootpkg "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/image_puller"
	unpackerpkg "code.cloudfoundry.org/grootfs/image_puller/unpacker"
	storepkg "code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/cache_driver"
	locksmithpkg "code.cloudfoundry.org/grootfs/store/locksmith"
	"code.cloudfoundry.org/grootfs/store/volume_driver"
	"code.cloudfoundry.org/lager"

	"code.cloudfoundry.org/commandrunner/linux_command_runner"
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
			Usage: "Set disk limit to be exclusive (i.e.: exluding image layers)",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("create")

		if ctx.NArg() != 2 {
			logger.Error("parsing-command", errors.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return cli.NewExitError(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		storePath := ctx.GlobalString("store")
		image := ctx.Args().First()
		id := ctx.Args().Tail()[0]

		diskLimit := ctx.Int64("disk-limit-size-bytes")
		if diskLimit < 0 {
			err := errors.New("invalid argument: disk limit cannot be negative")
			logger.Error("parsing-command", err)
			return cli.NewExitError(err.Error(), 1)
		}

		uidMappings, err := parseIDMappings(ctx.StringSlice("uid-mapping"))
		if err != nil {
			err = fmt.Errorf("parsing uid-mapping: %s", err)
			logger.Error("parsing-command", err)
			return cli.NewExitError(err.Error(), 1)
		}
		gidMappings, err := parseIDMappings(ctx.StringSlice("gid-mapping"))
		if err != nil {
			err = fmt.Errorf("parsing gid-mapping: %s", err)
			logger.Error("parsing-command", err)
			return cli.NewExitError(err.Error(), 1)
		}

		btrfsVolumeDriver := volume_driver.NewBtrfs(storePath)
		bundler := storepkg.NewBundler(btrfsVolumeDriver, storePath)

		runner := linux_command_runner.New()
		idMapper := unpackerpkg.NewIDMapper(runner)
		namespacedCmdUnpacker := unpackerpkg.NewNamespacedCmdUnpacker(runner, idMapper, "unpack")

		trustedRegistries := ctx.StringSlice("insecure-registry")
		dockerSrc := remote.NewDockerSource(trustedRegistries)

		cacheDriver := cache_driver.NewCacheDriver(storePath)
		remoteFetcher := remote.NewRemoteFetcher(dockerSrc, cacheDriver)

		localFetcher := local.NewLocalFetcher()

		parsedImageURL, err := url.Parse(image)
		if err != nil {
			err = fmt.Errorf("invalid image: %s", err)
			return cli.NewExitError(err.Error(), 1)
		}

		var imageFetcher image_puller.Fetcher
		if parsedImageURL.Scheme == "" {
			imageFetcher = localFetcher
		} else {
			imageFetcher = remoteFetcher
		}

		locksmith := locksmithpkg.NewFileSystem(storePath)
		imgPuller := image_puller.NewImagePuller(imageFetcher, namespacedCmdUnpacker, btrfsVolumeDriver)
		groot := grootpkg.IamGroot(bundler, imgPuller, locksmith)

		bundle, err := groot.Create(logger, grootpkg.CreateSpec{
			ID:                    id,
			Image:                 image,
			DiskLimit:             diskLimit,
			ExcludeImageFromQuota: ctx.Bool("exclude-image-from-quota"),
			UIDMappings:           uidMappings,
			GIDMappings:           gidMappings,
		})
		if err != nil {
			logger.Error("creating", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fmt.Println(bundle.Path())
		return nil
	},
}

func parseIDMappings(args []string) ([]grootpkg.IDMappingSpec, error) {
	mappings := []grootpkg.IDMappingSpec{}

	for _, v := range args {
		var mapping grootpkg.IDMappingSpec
		_, err := fmt.Sscanf(v, "%d:%d:%d", &mapping.NamespaceID, &mapping.HostID, &mapping.Size)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}
