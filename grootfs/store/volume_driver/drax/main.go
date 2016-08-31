package main

import (
	"os"

	"code.cloudfoundry.org/grootfs/store/volume_driver/drax/commands"

	"github.com/urfave/cli"
)

func main() {
	drax := cli.NewApp()
	drax.Name = "drax"
	drax.Usage = "The destroyer"
	drax.Version = "0.1.0"

	drax.Before = func(ctx *cli.Context) error {
		cli.ErrWriter = os.Stdout

		return nil
	}

	drax.Commands = []cli.Command{
		commands.LimitCommand,
		commands.DestroyCommand,
	}

	drax.Run(os.Args)
}
