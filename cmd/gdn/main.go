package main

import (
	"flag"
	"fmt"
	"os"

	"code.cloudfoundry.org/guardian/guardiancmd"

	"github.com/jessevdk/go-flags"
)

// version is overwritten at compile time by passing
// -ldflags -X main.version=<version>
var version = "dev"

func main() {
	configFilePath := flag.String("config", "", "config file path")
	printVersion := flag.Bool("v", false, "print version")
	flag.Parse()

	if *printVersion {
		fmt.Println(version)
		os.Exit(0)
	}

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
