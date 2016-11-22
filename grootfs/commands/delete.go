package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"errors"
	"fmt"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/idfinder"
	"code.cloudfoundry.org/grootfs/commands/storepath"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/dependency_manager"
	imageClonerpkg "code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/grootfs/store/volume_driver"
	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"
)

var DeleteCommand = cli.Command{
	Name:        "delete",
	Usage:       "delete <id|image path>",
	Description: "Deletes a container image",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("delete")

		var metricsEmitter groot.MetricsEmitter
		ctxMetricsEmitter := ctx.App.Metadata["metricsEmitter"]
		// We have to check nil like that because of how interfaces and reflection
		// work. It turns out that ctxMetricsEmitter is at this point of type
		// interface{} and an interface stores two pieces of information: the
		// actual value and the type.  Therefore, a simple `ctxMetricsEmitter ==
		// nil` does not really do the trick because plain `nil` is untyped.
		if ctxMetricsEmitter != (*metrics.Emitter)(nil) {
			metricsEmitter = ctxMetricsEmitter.(groot.MetricsEmitter)
		}

		storePath := ctx.GlobalString("store")
		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errors.New("id was not specified"))
			return cli.NewExitError("id was not specified", 1)
		}

		idOrPath := ctx.Args().First()
		id, err := idfinder.FindID(storePath, idOrPath)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		if id == idOrPath {
			storePath = storepath.UserBased(storePath)
		} else {
			storePath, err = idfinder.FindSubStorePath(storePath, idOrPath)
			if err != nil {
				return cli.NewExitError(fmt.Sprintf("can't determine the store path: %s", err.Error()), 1)
			}
		}

		btrfsVolumeDriver := volume_driver.NewBtrfs(ctx.GlobalString("drax-bin"), storePath)
		imageCloner := imageClonerpkg.NewImageCloner(btrfsVolumeDriver, storePath)
		dependencyManager := dependency_manager.NewDependencyManager(
			filepath.Join(storePath, store.META_DIR_NAME, "dependencies"),
		)
		deleter := groot.IamDeleter(imageCloner, dependencyManager, metricsEmitter)

		err = deleter.Delete(logger, id)
		if err != nil {
			logger.Error("deleting-image-failed", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fmt.Printf("Image %s deleted\n", id)
		return nil
	},
}
