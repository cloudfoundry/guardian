package commands // import "code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis/commands"

import (
	"github.com/docker/docker/pkg/system"
	errorspkg "github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var HandleOpqWhiteoutsCommand = cli.Command{
	Name:        "handle-opqwhiteouts",
	Usage:       "handle-opqwhiteouts --opaque-path <path>",
	Description: "Handle opaque whiteouts for a volume",

	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "opaque-path",
			Usage: "Path to whiteout file",
		},
	},

	Action: func(ctx *cli.Context) error {
		for _, opaqueWhiteout := range ctx.StringSlice("opaque-path") {
			if err := system.Lsetxattr(opaqueWhiteout, "trusted.overlay.opaque", []byte("y"), 0); err != nil {
				return errorspkg.Wrapf(err, "set xattr for %s", opaqueWhiteout)
			}
		}
		return nil
	},
}
