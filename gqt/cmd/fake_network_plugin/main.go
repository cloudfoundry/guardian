package main

import (
	"fmt"
	"io"
	"os"

	flag "github.com/spf13/pflag"
)

func main() {
	argsFilePath := flag.String("args-file", "", "")
	stdinFilePath := flag.String("stdin-file", "", "")
	output := flag.String("output", "", "")
	action := flag.String("action", "", "")
	handle := flag.String("handle", "", "")
	failOnceIfExists := flag.String("fail-once-if-exists", "", "")

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
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}

		if err := os.WriteFile(*stdinFilePath, input, 0600); err != nil {
			panic(err)
		}
	}

	if *output != "" {
		fmt.Println(*output)
	}

	if *failOnceIfExists != "" && fileExists(*failOnceIfExists) {
		err := os.Remove(*failOnceIfExists)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to remove file %s: %s\n", *failOnceIfExists, err)
		}
		os.Exit(1)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
