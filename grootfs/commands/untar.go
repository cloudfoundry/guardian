package commands

import (
	"os"
	"os/exec"

	"github.com/urfave/cli"
)

var UntarCommand = cli.Command{
	Name:        "untar",
	Usage:       "untar <tar args>",
	Description: "Internal use only",
	Hidden:      true,

	Action: func(ctx *cli.Context) error {
		ctrlPipeR := os.NewFile(3, "/ctrl/pipe")
		ctrlPipeR.Read(make([]byte, 1))

		cmd := exec.Command("tar", ctx.Args()...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return cli.NewExitError("tar failed", 1)
		}

		return nil
	},
}
