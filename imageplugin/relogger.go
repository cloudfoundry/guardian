package imageplugin

import (
	"bytes"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/chug"
)

//go:generate counterfeiter -o imagepluginfakes/fake_logger.go . Logger
type Logger interface {
	Debug(action string, data ...lager.Data)
	Info(action string, data ...lager.Data)
	Error(action string, err error, data ...lager.Data)
}

type Relogger struct {
	destination Logger
}

func NewRelogger(destination Logger) Relogger {
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
	if len(entry.Raw) == 0 {
		return
	}
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
