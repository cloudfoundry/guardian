package main

import (
	"fmt"
	"io/ioutil"
	"os"

	flag "github.com/spf13/pflag"
)

func main() {
	var argsFilePath *string = flag.String("args-file", "", "")
	var stdinFilePath *string = flag.String("stdin-file", "", "")
	var output *string = flag.String("output", "", "")
	var action *string = flag.String("action", "", "")
	var handle *string = flag.String("handle", "", "")
	var failOnceIfExists *string = flag.String("fail-once-if-exists", "", "")

	flag.Parse()

	if *argsFilePath != "" {
		argsFile, err := os.OpenFile(*argsFilePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
		if err != nil {
			panic(err)
		}
		defer argsFile.Close()
		if _, err := fmt.Fprintf(argsFile, "--action %s --handle %s\n", *action, *handle); err != nil {
			panic(err)
		}
	}

	if *stdinFilePath != "" {
		input, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}

		if err := ioutil.WriteFile(*stdinFilePath, input, 0600); err != nil {
			panic(err)
		}
	}

	if *output != "" {
		fmt.Println(*output)
	}

	if *failOnceIfExists != "" && fileExists(*failOnceIfExists) {
		os.Remove(*failOnceIfExists)
		os.Exit(1)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
