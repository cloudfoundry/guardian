package commands

import (
	"fmt"

	"code.cloudfoundry.org/grootfs/graph"
	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"
)

var CreateCommand = cli.Command{
	Name:        "create",
	Usage:       "create --image <image>",
	Description: "Creates a root filesystem for the provided image.",

	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "image, i",
			Value: "",
			Usage: "Local path or URL to the image",
		},
	},

	Action: func(ctx *cli.Context) error {
		graphPath := ctx.GlobalString("graph")
		imagePath := ctx.String("image")

		grph := graph.NewGraph(graphPath)
		logger := ctx.App.Metadata["logger"].(lager.Logger)

		bundlePath, err := grph.MakeBundle(logger, imagePath)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		fmt.Println(bundlePath)
		return nil
	},
}
