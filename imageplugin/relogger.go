package imageplugin

import (
	"bytes"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/chug"
)

//go:generate counterfeiter -o imagepluginfakes/fake_logger.go ../vendor/code.cloudfoundry.org/lager Logger

type Relogger struct {
	destination lager.Logger
}

func NewRelogger(destination lager.Logger) Relogger {
	return Relogger{
		destination: destination,
	}
}

func (r Relogger) Write(data []byte) (n int, err error) {
	entries := make(chan chug.Entry)

	go chug.Chug(bytes.NewReader(data), entries)

	for entry := range entries {
		r.relogEntry(entry)
	}

	return len(data), nil
}

func (r Relogger) relogEntry(entry chug.Entry) {
	if !entry.IsLager {
		r.destination.Error("error", nil, map[string]interface{}{"output": string(entry.Raw)})
		return
	}

	logEntry := entry.Log

	data := logEntry.Data
	data["original_timestamp"] = logEntry.Timestamp

	switch logEntry.LogLevel {
	case lager.DEBUG:
		r.destination.Debug(logEntry.Message, data)
	case lager.INFO:
		r.destination.Info(logEntry.Message, data)
	case lager.ERROR, lager.FATAL:
		r.destination.Error(logEntry.Message, logEntry.Error, data)
	}
}
