package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"fmt"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"
)

var ListCommand = cli.Command{
	Name:        "list",
	Usage:       "list",
	Description: "Lists images in store",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("list")

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		cfg := configBuilder.Build()
		logger.Debug("list-config", lager.Data{"currentConfig": cfg})
		storePath := cfg.BaseStorePath

		lister := groot.IamLister()
		images, err := lister.List(logger, storePath)
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("Failed to retrieve list of images: %s", err.Error()), 1)
		}

		if len(images) == 0 {
			fmt.Println("Store empty")
		}
		for _, image := range images {
			fmt.Println(image)
		}

		return nil
	},
}
