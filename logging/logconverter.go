package logging

import (
	"bytes"
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/lager"
)

type logLine struct {
	Msg   string
	Level string
}

func ForwardRuncLogsToLager(log lager.Logger, tag string, logfileContent []byte) {
	lines := bytes.Split(logfileContent, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var parsedLine logLine
		if err := json.Unmarshal(line, &parsedLine); err != nil {
			log.Info("error-parsing-runc-log-file", lager.Data{"message": string(line)})
			continue
		}
		log.Debug(tag, lager.Data{"message": parsedLine.Msg})
	}
}

type WrappedError struct {
	tag             string
	lastRuncLogLine string
	Underlying      error
}

func (e WrappedError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.tag, e.Underlying, e.lastRuncLogLine)
}

func WrapWithErrorFromLastLogLine(tag string, originalError error, logfileContent []byte) error {
	return WrappedError{Underlying: originalError, tag: tag, lastRuncLogLine: MsgFromLastLogLine(logfileContent)}
}

func MsgFromLastLogLine(logFileContent []byte) string {
	lines := bytes.Split(logFileContent, []byte("\n"))

	for i := len(lines) - 1; i >= 0; i-- {
		if len(lines[i]) == 0 {
			continue
		}

		var logLine logLine
		err := json.Unmarshal(lines[i], &logLine)
		if err != nil {
			continue
		}

		if logLine.Level == "error" || logLine.Level == "fatal" {
			return string(logLine.Msg)
		}
	}

	return ""
}
