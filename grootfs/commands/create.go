package commands

import (
	"errors"
	"fmt"

	clonerpkg "code.cloudfoundry.org/grootfs/cloner"
	streamerpkg "code.cloudfoundry.org/grootfs/cloner/streamer"
	unpackerpkg "code.cloudfoundry.org/grootfs/cloner/unpacker"
	fetcherpkg "code.cloudfoundry.org/grootfs/fetcher"
	storepkg "code.cloudfoundry.org/grootfs/store"
	grootpkg "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	"github.com/urfave/cli"
)

var CreateCommand = cli.Command{
	Name:        "create",
	Usage:       "create --image <image> <id>",
	Description: "Creates a root filesystem for the provided image.",

	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "image",
			Usage: "Local path or URL to the image",
		},
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

		storePath := ctx.GlobalString("store")
		image := ctx.String("image")
		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errors.New("id was not specified"))
			return cli.NewExitError("id was not specified", 1)
		}
		id := ctx.Args().First()
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

		store := storepkg.NewStore(storePath)

		runner := linux_command_runner.New()
		idMapper := unpackerpkg.NewIDMapper(runner)
		namespacedCmdUnpacker := unpackerpkg.NewNamespacedCmdUnpacker(runner, idMapper, "unpack")
		tarStreamer := streamerpkg.NewTarStreamer()

		localCloner := clonerpkg.NewLocalCloner(tarStreamer, namespacedCmdUnpacker)

		remoteFetcher := fetcherpkg.NewFetcher()
		remoteCloner := clonerpkg.NewRemoteCloner(remoteFetcher, namespacedCmdUnpacker)

		groot := grootpkg.IamGroot(store, localCloner, remoteCloner)

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
