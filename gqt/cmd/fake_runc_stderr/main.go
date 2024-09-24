package main

import "os"

func main() {
	stderrContents := ""
	for i := 0; i < 5000; i++ {
		stderrContents += "I am a bad runC\n"
	}
	// #nosec G104 -if we can't write to stderr, we can't write an error to stderr about how this failed. ignore the err
	os.Stderr.WriteString(stderrContents)
	os.Exit(100)
}
