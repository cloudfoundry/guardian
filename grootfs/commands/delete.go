package commands

import (
	"errors"
	"fmt"

	"code.cloudfoundry.org/grootfs/graph"
	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"
)

var DeleteCommand = cli.Command{
	Name:        "delete",
	Usage:       "delete <id>",
	Description: "Deletes a container bundle",

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)

		graphPath := ctx.GlobalString("graph")
		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errors.New("id was not specified"))
			return cli.NewExitError("id was not specified", 1)
		}
		id := ctx.Args().First()

		grp := graph.NewGraph(graphPath)
		err := grp.DeleteBundle(logger, id)
		if err != nil {
			logger.Error("deleting-bundle", err)
			return cli.NewExitError(err.Error(), 1)
		}

		fmt.Printf("Bundle %s deleted\n", id)
		return nil
	},
}
