package logging

import (
	"bytes"
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/lager/v3"
)

type LogLine struct {
	Msg   string
	Level string
}

func (l LogLine) IsError() bool {
	return l.Level == "error" || l.Level == "fatal"
}

func ForwardRuncLogsToLager(log lager.Logger, tag string, logfileContent []byte) LogLine {
	lastErrorLine := LogLine{}
	lines := bytes.Split(logfileContent, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var parsedLine LogLine
		if err := json.Unmarshal(line, &parsedLine); err != nil {
			log.Info("error-parsing-runc-log-file", lager.Data{"message": string(line), "error": err.Error()})
			continue
		}
		log.Debug(tag, lager.Data{"message": parsedLine.Msg})
		if parsedLine.IsError() {
			lastErrorLine = parsedLine
		}
	}

	return lastErrorLine
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
	return WrapWithErrorFromLastMessage(tag, originalError, MsgFromLastLogLine(logfileContent))
}

func WrapWithErrorFromLastMessage(tag string, originalError error, lastMessage string) error {
	return WrappedError{Underlying: originalError, tag: tag, lastRuncLogLine: lastMessage}
}

func MsgFromLastLogLine(logFileContent []byte) string {
	lines := bytes.Split(logFileContent, []byte("\n"))

	for i := len(lines) - 1; i >= 0; i-- {
		if len(lines[i]) == 0 {
			continue
		}

		var logLine LogLine
		err := json.Unmarshal(lines[i], &logLine)
		if err != nil {
			continue
		}

		if logLine.IsError() {
			return string(logLine.Msg)
		}
	}

	return ""
}
