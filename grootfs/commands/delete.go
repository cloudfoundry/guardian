package commands

import (
	"errors"
	"fmt"

	grootpkg "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/volume_driver"
	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"
)

var DeleteCommand = cli.Command{
	Name:        "delete",
	Usage:       "delete <id>",
	Description: "Deletes a container bundle",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("delete")

		storePath := ctx.GlobalString("store")
		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errors.New("id was not specified"))
			return cli.NewExitError("id was not specified", 1)
		}
		id := ctx.Args().First()

		btrfsVolumeDriver := volume_driver.NewBtrfs(ctx.GlobalString("drax-bin"), storePath)

		bundler := store.NewBundler(btrfsVolumeDriver, storePath)
		groot := grootpkg.IamGroot(bundler, nil, nil)

		err := groot.Delete(logger, id)
		if err != nil {
			logger.Error("deleting-bundle-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fmt.Printf("Bundle %s deleted\n", id)
		return nil
	},
}
