package commands

import (
	"errors"
	"fmt"

	"code.cloudfoundry.org/grootfs/graph"
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

		grph := graph.NewGraph(graphPath)

		bundlePath, err := grph.MakeBundle(logger, imagePath, id)
		if err != nil {
			logger.Error("making-bundle", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fmt.Println(bundlePath)
		return nil
	},
}
