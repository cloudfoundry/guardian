package runrunc

import (
	"bytes"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/go-logfmt/logfmt"
)

func ForwardRuncLogsToLager(log lager.Logger, logfileContent []byte) {
	decoder := logfmt.NewDecoder(bytes.NewReader(logfileContent))
	for decoder.ScanRecord() {
		for decoder.ScanKeyval() {
			if string(decoder.Key()) == "msg" {
				log.Debug("runc", lager.Data{"message": string(decoder.Value())})
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

func WrapWithErrorFromLastLogLine(originalError error, logfileContent []byte) error {
	lastLogLine := lastNonEmptyLine(logfileContent)
	decoder := logfmt.NewDecoder(bytes.NewReader(lastLogLine))
	if decoder.ScanRecord() {
		for decoder.ScanKeyval() {
			if string(decoder.Key()) == "msg" {
				return fmt.Errorf("runc: %s: %s", originalError, string(decoder.Value()))
			}
		}
		if err := decoder.Err(); err != nil {
			return fmt.Errorf("runc: %s: %s", originalError, string(logfileContent))
		}
	} else {
		return fmt.Errorf("runc: %s: %s", originalError, string(logfileContent))
	}

	return nil
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
