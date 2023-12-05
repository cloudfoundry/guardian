package commands

import (
	"code.cloudfoundry.org/lager/v3"
	"io"
)

func createLoggingSink(writer io.Writer, logLevel lager.LogLevel, logFormat string) lager.Sink {
	if logFormat == "rfc3339" {
		return lager.NewPrettySink(writer, logLevel)
	}
	return lager.NewWriterSink(writer, logLevel)
}
