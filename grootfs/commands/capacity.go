package commands // import "code.cloudfoundry.org/grootfs/commands"

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/lager/v3"
	"github.com/urfave/cli/v2"
)

var CapacityCommand = cli.Command{
	Name:        "capacity",
	Usage:       "capacity",
	Description: "Return filesystem capacity",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)
		logger = logger.Session("capacity")

		configBuilder := ctx.App.Metadata["configBuilder"].(*config.Builder)
		cfg, err := configBuilder.Build()
		logger.Debug("capacity-config", lager.Data{"currentConfig": cfg})
		if err != nil {
			logger.Error("config-builder-failed", err)
			return cli.Exit(err.Error(), 1)
		}

		_ = json.NewEncoder(os.Stdout).Encode(map[string]uint64{
			"capacity": uint64(cfg.Init.StoreSizeBytes),
		})

		return nil
	},
}
