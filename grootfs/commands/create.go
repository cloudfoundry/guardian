package commands

import (
	"errors"
	"fmt"

	clonerpkg "code.cloudfoundry.org/grootfs/cloner"
	graphpkg "code.cloudfoundry.org/grootfs/graph"
	grootpkg "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"
)

var CreateCommand = cli.Command{
	Name:        "create",
	Usage:       "create --image <image> <id>",
	Description: "Creates a root filesystem for the provided image.",

	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "image, i",
			Value: "",
			Usage: "Local path or URL to the image",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)

		graphPath := ctx.GlobalString("graph")
		imagePath := ctx.String("image")
		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errors.New("id was not specified"))
			return cli.NewExitError("id was not specified", 1)
		}
		id := ctx.Args().First()

		graph := graphpkg.NewGraph(graphPath)
		cloner := clonerpkg.NewTarCloner()
		groot := grootpkg.IamGroot(graph, cloner)

		bundlePath, err := groot.Create(logger, grootpkg.CreateSpec{
			ID:        id,
			ImagePath: imagePath,
		})
		if err != nil {
			logger.Error("making-bundle", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fmt.Println(bundlePath)
		return nil
	},
}
