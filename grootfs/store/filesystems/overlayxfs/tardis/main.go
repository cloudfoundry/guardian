package main

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis/commands"

	"github.com/urfave/cli/v2"
)

func main() {
	tardis := cli.NewApp()
	tardis.Name = "tardis"
	tardis.Usage = "The destroyer"
	tardis.Version = "0.1.0"

	tardis.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "log-timestamp-format",
			Usage: "The format to use for timestamps in logs. Can be 'unix-epoch' or 'rfc3339'",
		},
	}

	tardis.Before = func(ctx *cli.Context) error {
		cli.ErrWriter = os.Stdout

		return nil
	}

	tardis.Commands = []*cli.Command{
		&commands.LimitCommand,
		&commands.StatsCommand,
		&commands.HandleOpqWhiteoutsCommand,
	}

	err := tardis.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
		os.Exit(1)
	}
}
