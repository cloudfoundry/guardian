package commands // import "code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis/commands"

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	quotapkg "code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/quota"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis/ids"
	"code.cloudfoundry.org/lager/v3"
	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var LimitCommand = cli.Command{
	Name:        "limit",
	Usage:       "limit --disk-limit-bytes 102400 --image-path <path>",
	Description: "Add disk limits to the volume.",

	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "image-path",
			Usage: "Path to the volume",
		},
		&cli.Int64Flag{
			Name:  "disk-limit-bytes",
			Usage: "Disk limit in bytes",
		},
	},

	Action: func(ctx *cli.Context) error {
		logger := lager.NewLogger("tardis")
		logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
		logger.RegisterSink(lager.NewPrettySink(os.Stderr, lager.ERROR))
		logger.Info("starting")
		defer logger.Info("ending")

		imagePath := ctx.String("image-path")
		imagesPath := filepath.Dir(imagePath)

		diskLimit := uint64(ctx.Int64("disk-limit-bytes"))
		idDiscoverer := ids.NewDiscoverer(filepath.Join(filepath.Dir(imagesPath), overlayxfs.IDDir))
		projectID, err := idDiscoverer.Alloc(logger)
		if err != nil {
			logger.Error("allocating-project-id", err)
			return errorspkg.Wrap(err, "allocating project id")
		}

		return func(logger lager.Logger) error {
			logger = logger.Session("set-quota")
			logger.Debug("starting")
			defer logger.Debug("ending")

			if err := quotapkg.Set(logger, projectID, imagePath, diskLimit); err != nil {
				logger.Error("setting-quota-failed", err)
				return errorspkg.Wrapf(err, "setting quota to %s", imagePath)
			}
			return nil
		}(logger)
	},
}
