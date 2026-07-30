package relogger

import (
	"bytes"
	"io"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/chug"
)

type Relogger struct {
	dest lager.Logger
}

func NewRelogger(destination lager.Logger) *Relogger {
	return &Relogger{
		dest: destination,
	}
}

func (r *Relogger) Write(data []byte) (n int, err error) {
	RelogBytes(r.dest, data)
	return len(data), nil
}

func RelogBytes(destination lager.Logger, source []byte) {
	buffer := bytes.NewBuffer(source)
	RelogStream(destination, buffer)
}

func RelogStream(destination lager.Logger, input io.Reader) {
	entries := make(chan chug.Entry)
	go chug.Chug(input, entries)
	for entry := range entries {
		if entry.IsLager {
			relog(destination, entry.Log)
		}
	}
}

func relog(logger lager.Logger, entry chug.LogEntry) {
	data := entry.Data
	data["original_timestamp"] = entry.Timestamp

	switch entry.LogLevel {
	case lager.DEBUG:
		logger.Debug(entry.Message, data)
	case lager.INFO:
		logger.Info(entry.Message, data)
	case lager.ERROR, lager.FATAL:
		logger.Error(entry.Message, entry.Error, data)
	}
}
