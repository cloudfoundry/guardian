package main

import (
	"flag"
	"fmt"
	"os"

	"code.cloudfoundry.org/guardian/guardiancmd"

	"github.com/jessevdk/go-flags"
)

func main() {
	configFilePath := flag.String("config", "", "config file path")
	flag.Parse()

	cmd := &guardiancmd.GdnCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	if *configFilePath != "" {
		iniParser := flags.NewIniParser(parser)
		must(iniParser.ParseFile(*configFilePath))
	}

	_, err := parser.Parse()
	mustNot(err)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var mustNot = must
