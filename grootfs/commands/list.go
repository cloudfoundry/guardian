package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager/v3"
	errorspkg "github.com/pkg/errors"

	"github.com/urfave/cli/v2"
)

var ListCommand = cli.Command{
	Name:        "list",
	Usage:       "list",
	Description: "Lists images in store",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("list")

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		cfg, err := configBuilder.Build()
		logger.Debug("list-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		if _, err := os.Stat(cfg.StorePath); os.IsNotExist(err) {
			err := errorspkg.Errorf("no store found at %s", cfg.StorePath)
			logger.Error("store-path-failed", err, nil)
			return cli.NewExitError(err.Error(), 1)
		}

		lister := groot.IamLister()
		images, err := lister.List(logger, cfg.StorePath)
		if err != nil {
			logger.Error("listing-images", err, lager.Data{"storePath": cfg.StorePath})
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
