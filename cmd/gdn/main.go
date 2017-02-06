package main

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/guardian/guardiancmd"

	"github.com/jessevdk/go-flags"
)

func main() {
	// TODO: Hmmmm think about what to do here
	args := os.Args[1:]
	if args[0] != "setup" && args[0] != "server" {
		args = append([]string{"server"}, args...)
		setupCmd := &guardiancmd.SetupCommand{}
		if err := setupCmd.Execute(nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	cmd := &guardiancmd.GdnCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	if _, err := parser.ParseArgs(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
