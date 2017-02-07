package main

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/guardian/guardiancmd"

	"github.com/jessevdk/go-flags"
)

func main() {
	cmd := &guardiancmd.GdnCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	if _, err := parser.Parse(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}
