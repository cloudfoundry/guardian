package commands

import (
	"errors"
	"fmt"
	"path/filepath"

	clonerpkg "code.cloudfoundry.org/grootfs/cloner"
	streamerpkg "code.cloudfoundry.org/grootfs/cloner/streamer"
	unpackerpkg "code.cloudfoundry.org/grootfs/cloner/unpacker"
	fetcherpkg "code.cloudfoundry.org/grootfs/fetcher"
	grootpkg "code.cloudfoundry.org/grootfs/groot"
	storepkg "code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/cache_driver"
	"code.cloudfoundry.org/grootfs/store/volume_driver"
	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	"github.com/containers/image/types"
	"github.com/urfave/cli"
)

var CreateCommand = cli.Command{
	Name:        "create",
	Usage:       "create [options] <image> <id>",
	Description: "Creates a root filesystem for the provided image.",

	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "uid-mapping",
			Usage: "UID mapping for image translation, e.g.: <Namespace UID>:<Host UID>:<Size>",
		},
		cli.StringSliceFlag{
			Name:  "gid-mapping",
			Usage: "GID mapping for image translation, e.g.: <Namespace UID>:<Host UID>:<Size>",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("create")

		if ctx.NArg() != 2 {
			logger.Error("parsing-command", errors.New("invalid arguments"))
			return cli.NewExitError("invalid arguments", 1)
		}

		storePath := ctx.GlobalString("store")
		image := ctx.Args().First()
		id := ctx.Args().Tail()[0]

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
		bundler := storepkg.NewBundler(storePath)

		runner := linux_command_runner.New()
		idMapper := unpackerpkg.NewIDMapper(runner)
		namespacedCmdUnpacker := unpackerpkg.NewNamespacedCmdUnpacker(runner, idMapper, "unpack")

		tarStreamer := streamerpkg.NewTarStreamer()

		localCloner := clonerpkg.NewLocalCloner(tarStreamer, namespacedCmdUnpacker, btrfsVolumeDriver)

		cachePath := filepath.Join(storePath, "cache", "blobs")
		cacheDriver := cache_driver.NewCacheDriver(storePath)
		remoteFetcher := fetcherpkg.NewFetcher(cachePath, func(ref types.ImageReference) fetcherpkg.Image {
			return fetcherpkg.NewContainersImage(ref, cacheDriver)
		})

		remoteCloner := clonerpkg.NewRemoteCloner(remoteFetcher, namespacedCmdUnpacker, btrfsVolumeDriver)

		groot := grootpkg.IamGroot(bundler, localCloner, remoteCloner, btrfsVolumeDriver)

		bundle, err := groot.Create(logger, grootpkg.CreateSpec{
			ID:          id,
			Image:       image,
			UIDMappings: uidMappings,
			GIDMappings: gidMappings,
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
