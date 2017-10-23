package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/store/filesystems"
	"code.cloudfoundry.org/lager"

	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli"
)

var GenerateVolumeSizeMetadata = cli.Command{
	Name:   "generate-volume-size-metadata",
	Hidden: true,

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("generate-metadata")

		if ctx.NArg() != 0 {
			logger.Error("parsing-command", errorspkg.New("invalid arguments"), lager.Data{"args": ctx.Args()})
			return cli.NewExitError(fmt.Sprintf("invalid arguments - usage: %s", ctx.Command.Usage), 1)
		}

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		cfg, err := configBuilder.Build()
		if err != nil {
			return err
		}

		driver, err := createFileSystemDriver(cfg)
		if err != nil {
			return err
		}

		volumes, err := driver.Volumes(logger)
		if err != nil {
			return err
		}

		for _, volumeID := range volumes {
			_, err := driver.VolumeSize(logger, volumeID)
			if os.IsNotExist(errorspkg.Cause(err)) {
				logger.Info("volume-meta-missing", lager.Data{"volumeID": volumeID})

				volumePath, err := driver.VolumePath(logger, volumeID)
				if err != nil {
					return err
				}

				size, err := filesystems.CalculatePathSize(logger, volumePath)
				if err != nil {
					return err
				}

				err = driver.WriteVolumeMeta(logger, volumeID, base_image_puller.VolumeMeta{Size: size})
				if err != nil {
					return err
				}
			}
		}

		return nil
	},
}
