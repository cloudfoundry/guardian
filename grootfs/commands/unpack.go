package commands

import (
	"errors"
	"fmt"
	"os"

	"code.cloudfoundry.org/grootfs/cloner"
	unpackerpkg "code.cloudfoundry.org/grootfs/cloner/unpacker"
	"code.cloudfoundry.org/lager"

	"github.com/urfave/cli"
)

var UnpackCommand = cli.Command{
	Name:        "unpack",
	Usage:       "unpack <destination directory>",
	Description: "Internal use only",
	Hidden:      true,

	Action: func(ctx *cli.Context) error {
		logger := ctx.App.Metadata["logger"].(lager.Logger)

		if ctx.NArg() != 1 {
			logger.Error("parsing-command", errors.New("destination directory was not specified"))
			return cli.NewExitError("destination directory was not specified", 1)
		}
		rootFSPath := ctx.Args().Get(0)

		ctrlPipeR := os.NewFile(3, "/ctrl/pipe")
		buffer := make([]byte, 1)
		_, err := ctrlPipeR.Read(buffer)
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("reading from control pipe: %s", err), 1)
		}

		unpacker := unpackerpkg.NewTarUnpacker()
		if err := unpacker.Unpack(logger, cloner.UnpackSpec{
			Stream:     os.Stdin,
			RootFSPath: rootFSPath,
		}); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		return nil
	},
}
