package main

import (
	"os"

	"github.com/Sirupsen/logrus"
)

func main() {
	logPath := ""
	for idx, s := range os.Args {
		if s == "-log" || s == "--log" {
			logPath = os.Args[idx+1]
			break
		}
	}

	f, err := os.Create(logPath)
	if err != nil {
		os.Exit(1)
	}

	logrus.SetOutput(f)
	logrus.Info("guardian-runc-logging-test-info")
	logrus.Warn("guardian-runc-logging-test-warn")
	logrus.Error("guardian-runc-logging-test-error")
	logrus.Print("guardian-runc-logging-test-print")
}
