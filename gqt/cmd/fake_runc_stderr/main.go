package main

import "os"

func main() {
	stderrContents := ""
	for i := 0; i < 5000; i++ {
		stderrContents += "I am a bad runC\n"
	}
	os.Stderr.WriteString(stderrContents)
	os.Exit(100)
}
