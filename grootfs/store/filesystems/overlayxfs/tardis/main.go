package main

import (
	"os"

	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis/commands"

	"github.com/urfave/cli/v2"
)

func main() {
	tardis := cli.NewApp()
	tardis.Name = "tardis"
	tardis.Usage = "The destroyer"
	tardis.Version = "0.1.0"

	tardis.Before = func(ctx *cli.Context) error {
		cli.ErrWriter = os.Stdout

		return nil
	}

	tardis.Commands = []*cli.Command{
		&commands.LimitCommand,
		&commands.StatsCommand,
		&commands.HandleOpqWhiteoutsCommand,
	}

	tardis.Run(os.Args)
}
