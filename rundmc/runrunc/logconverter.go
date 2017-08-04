package runrunc

import (
	"bytes"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/go-logfmt/logfmt"
)

func ForwardRuncLogsToLager(log lager.Logger, tag string, logfileContent []byte) {
	decoder := logfmt.NewDecoder(bytes.NewReader(logfileContent))
	for decoder.ScanRecord() {
		for decoder.ScanKeyval() {
			if string(decoder.Key()) == "msg" {
				log.Debug(tag, lager.Data{"message": string(decoder.Value())})
			}
		}
		if err := decoder.Err(); err != nil {
			writeWholeLogfileToLager(log, logfileContent)
			return
		}
	}
	if err := decoder.Err(); err != nil {
		writeWholeLogfileToLager(log, logfileContent)
		return
	}
}

func WrapWithErrorFromLastLogLine(tag string, originalError error, logfileContent []byte) error {
	lastLogLine := lastNonEmptyLine(logfileContent)
	decoder := logfmt.NewDecoder(bytes.NewReader(lastLogLine))
	if decoder.ScanRecord() {
		for decoder.ScanKeyval() {
			if string(decoder.Key()) == "msg" {
				return fmt.Errorf("%s: %s: %s", tag, originalError, string(decoder.Value()))
			}
		}
		return fmt.Errorf("%s: %s: %s", tag, originalError, string(logfileContent))
	} else {
		return fmt.Errorf("%s: %s: %s", tag, originalError, string(logfileContent))
	}
}

func lastNonEmptyLine(content []byte) []byte {
	lines := bytes.Split(content, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		if string(lines[i]) != "" {
			return lines[i]
		}
	}
	return lines[len(lines)-1]
}

func writeWholeLogfileToLager(log lager.Logger, logfileContent []byte) {
	log.Info("error-parsing-runc-log-file", lager.Data{"message": string(logfileContent)})
}
