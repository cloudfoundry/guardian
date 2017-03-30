package commands // import "code.cloudfoundry.org/grootfs/store/filesystems/btrfs/drax/commands"

import (
	"os"
	"strings"

	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	"code.cloudfoundry.org/grootfs/store/filesystems/btrfs/drax/lister"
	"code.cloudfoundry.org/lager"
	"github.com/urfave/cli"
)

var ListCommand = cli.Command{
	Name:        "list",
	Usage:       "list <path>",
	Description: "List btrfs subvolumes inside the path",

	Flags: []cli.Flag{},

	Action: func(ctx *cli.Context) error {
		logger := lager.NewLogger("drax")
		logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

		if len(ctx.Args()) != 1 {
			return cli.NewExitError("missing path argument", 1)
		}

		commandRunner := linux_command_runner.New()
		lister := lister.NewBtrfsLister(ctx.GlobalString("btrfs-bin"), commandRunner)
		volumes, err := lister.List(logger, ctx.Args()[0])
		if err != nil {
			logger.Error("listing-volumes", err)
			return cli.NewExitError(err.Error(), 1)
		}

		if _, err := os.Stdout.WriteString(strings.Join(volumes, "\n")); err != nil {
			logger.Error("encoding-stats", err)
			return cli.NewExitError(err.Error(), 1)
		}

		return nil
	},
}
