package logging

import (
	"bytes"
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/lager"
)

type logLine struct{ Msg string }

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
	msg := lastNonEmptyLine(logFileContent)
	var line logLine
	if err := json.Unmarshal(msg, &line); err == nil {
		msg = []byte(line.Msg)
	}

	return string(msg)
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
