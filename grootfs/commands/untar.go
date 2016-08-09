package commands

import (
	"errors"
	"os"

	unpackerpkg "code.cloudfoundry.org/grootfs/cloner/unpacker"
	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	"github.com/urfave/cli"
)

var UntarCommand = cli.Command{
	Name:        "untar",
	Usage:       "untar <tar args>",
	Description: "Internal use only",
	Hidden:      true,

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)

		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errors.New("destination directory was not specified"))
			return cli.NewExitError("destination directory was not specified", 1)
		}
		toDir := ctx.Args().Get(0)

		runner := linux_command_runner.New()
		unpacker := unpackerpkg.NewTarUnpacker(unpackerpkg.NewIDMapper(runner))

		ctrlPipeR := os.NewFile(3, "/ctrl/pipe")
		if err := unpacker.Untar(logger, ctrlPipeR, os.Stdin, toDir); err != nil {
			return cli.NewExitError("tar failed", 1)
		}

		return nil
	},
}
